package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"dnswatcher/backend/internal/contracts"
	"dnswatcher/backend/internal/policy"

	"github.com/miekg/dns"
)

var (
	errTimeout          = errors.New("timeout")
	errLoopDetected     = errors.New("loop detected")
	errMaxDepth         = errors.New("max depth exceeded")
	errUnusableReferral = errors.New("unusable referral")
)

type Config struct {
	PerHopTimeout      time.Duration
	OverallTimeout     time.Duration
	MaxDepth           int
	MaxUpstreamQueries int
	Roots              []ServerCandidate
	DestinationPolicy  policy.DestinationPolicy
	EndpointResolver   func(ip string) string
}

type ServerCandidate struct {
	name     string
	ip       string
	endpoint string
	zone     string
}

func NewServerCandidate(name, ip, zone, endpoint string) ServerCandidate {
	if endpoint == "" {
		endpoint = net.JoinHostPort(ip, "53")
	}
	return ServerCandidate{name: name, ip: ip, zone: zone, endpoint: endpoint}
}

type Engine struct {
	cfg Config
}

func NewEngine(cfg Config) *Engine {
	if cfg.PerHopTimeout == 0 {
		cfg.PerHopTimeout = 2500 * time.Millisecond
	}
	if cfg.OverallTimeout == 0 {
		cfg.OverallTimeout = 12 * time.Second
	}
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 20
	}
	if cfg.MaxUpstreamQueries == 0 {
		cfg.MaxUpstreamQueries = 40
	}
	if len(cfg.Roots) == 0 {
		cfg.Roots = DefaultRootCandidates()
	}
	if cfg.DestinationPolicy == nil {
		cfg.DestinationPolicy = policy.PublicIPPolicy{}
	}
	if cfg.EndpointResolver == nil {
		cfg.EndpointResolver = func(ip string) string {
			return net.JoinHostPort(ip, "53")
		}
	}
	for i := range cfg.Roots {
		if cfg.Roots[i].endpoint == "" {
			cfg.Roots[i].endpoint = cfg.EndpointResolver(cfg.Roots[i].ip)
		}
	}
	return &Engine{cfg: cfg}
}

func (e *Engine) Trace(ctx context.Context, req contracts.TraceRequest) (contracts.TraceResult, error) {
	startedAt := time.Now().UTC()
	ctx, cancel := context.WithTimeout(ctx, e.cfg.OverallTimeout)
	defer cancel()

	normalizedDomain, err := policy.NormalizeDomain(req.Domain)
	if err != nil {
		return contracts.TraceResult{}, err
	}
	qtype, err := policy.NormalizeQType(req.QType)
	if err != nil {
		return contracts.TraceResult{}, err
	}

	state := &traceState{
		traceID:          newTraceID(),
		inputDomain:      req.Domain,
		normalizedDomain: normalizedDomain,
		qtype:            qtype,
		startedAt:        startedAt,
		visited:          map[string]struct{}{},
	}

	outcome := e.traceName(ctx, state, fqdn(normalizedDomain), qtype, e.cfg.Roots, nil, "delegation", ".")
	state.result.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	state.result.StartedAt = startedAt.Format(time.RFC3339)
	state.result.TraceID = state.traceID
	state.result.InputDomain = req.Domain
	state.result.NormalizedDomain = normalizedDomain
	state.result.QType = qtype
	state.result.TotalDurationMS = int(time.Since(startedAt).Milliseconds())
	state.result.Hops = state.hops
	state.result.FinalOutcome = outcome
	if outcome.Kind == "success" {
		state.result.Status = "success"
	} else {
		state.result.Status = "failure"
	}
	answerCount := 0
	cnameCount := 0
	for _, hop := range state.hops {
		answerCount += len(hop.AnswerRRSets)
		if hop.ResponseKind == "cname" {
			cnameCount++
		}
	}
	state.result.Summary = contracts.TraceSummary{
		Headline:    outcome.Message,
		Detail:      summaryDetail(outcome.Kind, normalizedDomain, qtype),
		TotalHops:   len(state.hops),
		AnswerCount: answerCount,
		CNAMECount:  cnameCount,
	}
	state.result.TruthNotes = []contracts.TruthNote{
		{Code: "backend_trace_service", Message: "Traces are performed by the backend trace service."},
		{Code: "not_user_resolver_path", Message: "Timings reflect the backend service path, not your device's resolver path."},
		{Code: "live_results_vary", Message: "Repeated traces may differ because live DNS data and network conditions change."},
		{Code: "qname_minimization_deferred", Message: "V1 sends the full query name during iterative tracing; QNAME minimization is deferred."},
	}
	return state.result, nil
}

type traceState struct {
	traceID          string
	inputDomain      string
	normalizedDomain string
	qtype            string
	startedAt        time.Time
	result           contracts.TraceResult
	hops             []contracts.Hop
	visited          map[string]struct{}
	upstreamQueries  int
}

func (e *Engine) traceName(ctx context.Context, state *traceState, qname, qtype string, candidates []ServerCandidate, parent *int, purpose, zone string) contracts.FinalOutcome {
	currentCandidates := candidates
	currentQName := qname
	currentQType := qtype
	currentPurpose := purpose
	currentZone := zone

	for depth := 0; depth < e.cfg.MaxDepth; depth++ {
		if state.upstreamQueries >= e.cfg.MaxUpstreamQueries {
			return contracts.FinalOutcome{Kind: "max_depth", RCode: "BUDGET", Message: "The trace exceeded its upstream query budget.", TerminalHopIndex: lastHopIndex(state.hops)}
		}
		hop, response, nextCandidates, cnameTarget, err := e.queryCandidates(ctx, state, currentCandidates, currentQName, currentQType, currentPurpose, currentZone, parent)
		hop = normalizeHop(hop)
		state.hops = append(state.hops, hop)
		hopIndex := hop.Index
		switch {
		case err == nil && response != nil:
			switch hop.ResponseKind {
			case "answer":
				return contracts.FinalOutcome{Kind: "success", RCode: hop.ResponseCode, Message: "Authoritative answer returned.", TerminalHopIndex: hopIndex}
			case "nodata":
				return contracts.FinalOutcome{Kind: "success", RCode: hop.ResponseCode, Message: "The authoritative server returned no records of the requested type.", TerminalHopIndex: hopIndex}
			case "cname":
				currentQName = cnameTarget
				currentCandidates = e.cfg.Roots
				currentPurpose = "cname_follow"
				currentZone = "."
				continue
			case "referral":
				if len(nextCandidates) == 0 {
					return contracts.FinalOutcome{Kind: "unusable_referral", RCode: hop.ResponseCode, Message: "A referral was received, but no safe usable next target remained.", TerminalHopIndex: hopIndex}
				}
				currentCandidates = nextCandidates
				if purpose == "nameserver_address_lookup" {
					currentPurpose = "nameserver_address_lookup"
				} else {
					currentPurpose = "delegation"
				}
				currentZone = nextCandidates[0].zone
				continue
			default:
				return contracts.FinalOutcome{Kind: "success", RCode: hop.ResponseCode, Message: hop.Explanation, TerminalHopIndex: hopIndex}
			}
		case errors.Is(err, errTimeout):
			return contracts.FinalOutcome{Kind: "timeout", RCode: "TIMEOUT", Message: "All candidate upstream servers timed out before the trace could continue.", TerminalHopIndex: hopIndex}
		case errors.Is(err, errLoopDetected):
			return contracts.FinalOutcome{Kind: "loop_detected", RCode: "LOOP", Message: "The trace repeated the same lookup path and stopped to avoid a loop.", TerminalHopIndex: hopIndex}
		case errors.Is(err, errMaxDepth):
			return contracts.FinalOutcome{Kind: "max_depth", RCode: "MAX_DEPTH", Message: "The trace exceeded its maximum hop depth.", TerminalHopIndex: hopIndex}
		case errors.Is(err, errUnusableReferral):
			return contracts.FinalOutcome{Kind: "unusable_referral", RCode: hop.ResponseCode, Message: "A referral was received, but no safe usable next target remained.", TerminalHopIndex: hopIndex}
		default:
			switch hop.ResponseCode {
			case dns.RcodeToString[dns.RcodeNameError]:
				return contracts.FinalOutcome{Kind: "nxdomain", RCode: hop.ResponseCode, Message: "The authoritative server reported that the queried name does not exist.", TerminalHopIndex: hopIndex}
			case dns.RcodeToString[dns.RcodeServerFailure]:
				return contracts.FinalOutcome{Kind: "servfail", RCode: hop.ResponseCode, Message: "The upstream server reported a server failure.", TerminalHopIndex: hopIndex}
			case dns.RcodeToString[dns.RcodeRefused]:
				return contracts.FinalOutcome{Kind: "refused", RCode: hop.ResponseCode, Message: "The upstream server refused the query.", TerminalHopIndex: hopIndex}
			case dns.RcodeToString[dns.RcodeNotImplemented]:
				return contracts.FinalOutcome{Kind: "not_implemented", RCode: hop.ResponseCode, Message: "The upstream server does not implement the requested operation.", TerminalHopIndex: hopIndex}
			default:
				return contracts.FinalOutcome{Kind: "servfail", RCode: hop.ResponseCode, Message: hop.Explanation, TerminalHopIndex: hopIndex}
			}
		}
	}
	return contracts.FinalOutcome{Kind: "max_depth", RCode: "MAX_DEPTH", Message: "The trace exceeded its maximum hop depth.", TerminalHopIndex: lastHopIndex(state.hops)}
}

func (e *Engine) queryCandidates(ctx context.Context, state *traceState, candidates []ServerCandidate, qname, qtype, purpose, zone string, parent *int) (contracts.Hop, *dns.Msg, []ServerCandidate, string, error) {
	var lastHop contracts.Hop
	var timeoutCount int
	var lastErr error
	for _, candidate := range candidates {
		if state.upstreamQueries >= e.cfg.MaxUpstreamQueries {
			return lastHop, nil, nil, "", errMaxDepth
		}
		if err := e.cfg.DestinationPolicy.Allow(candidate.ip); err != nil {
			lastErr = errUnusableReferral
			continue
		}
		visitedKey := strings.Join([]string{candidate.ip, qname, qtype, purpose}, "|")
		if _, ok := state.visited[visitedKey]; ok {
			lastHop = newErrorHop(len(state.hops), parent, candidate, qname, qtype, purpose, zone, "LOOP", "error", "The trace repeated a previously queried server and stopped.", "Loop detection blocked a repeated lookup path.")
			return lastHop, nil, nil, "", errLoopDetected
		}
		state.visited[visitedKey] = struct{}{}
		state.upstreamQueries++
		response, latency, transport, truncated, err := e.exchange(ctx, candidate.endpoint, qname, qtype)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutCount++
				lastHop = newErrorHop(len(state.hops), parent, candidate, qname, qtype, purpose, zone, "TIMEOUT", "error", "The queried server did not respond before the timeout.", "The backend timed out waiting for the upstream server.")
				lastErr = errTimeout
				continue
			}
			lastHop = newErrorHop(len(state.hops), parent, candidate, qname, qtype, purpose, zone, "ERROR", "error", "The query failed before a usable DNS response was received.", err.Error())
			lastErr = err
			continue
		}
		hop := contracts.Hop{
			Index:            len(state.hops),
			ParentIndex:      parent,
			Role:             classifyRole(candidate.zone, purpose, response, qname, qtype),
			HopPurpose:       purpose,
			ZoneContext:      zone,
			ServerName:       candidate.name,
			ServerIP:         candidate.ip,
			QName:            qname,
			QType:            qtype,
			Transport:        transport,
			LatencyMS:        int(latency.Milliseconds()),
			ResponseCode:     dns.RcodeToString[response.Rcode],
			Authoritative:    response.Authoritative,
			Truncated:        truncated,
			AnswerRRSets:     rrsetsFromSection("answer", response.Answer),
			AuthorityRRSets:  rrsetsFromSection("authority", response.Ns),
			AdditionalRRSets: rrsetsFromSection("additional", response.Extra),
		}
		cnameTarget := firstCNAME(response.Answer, qname)
		if response.Rcode != dns.RcodeSuccess {
			hop.ResponseKind = "error"
			hop.Explanation = explainRCode(response.Rcode)
			hop.TechnicalNote = "Non-success response code ended the trace."
			return hop, response, nil, "", fmt.Errorf("rcode: %s", hop.ResponseCode)
		}
		if cnameTarget != "" && !hasType(response.Answer, qname, qtype) {
			hop.ResponseKind = "cname"
			hop.Explanation = "The server returned a CNAME target, so the trace must continue for the alias."
			hop.TechnicalNote = "The backend restarts iterative tracing from the root for the CNAME target."
			return hop, response, nil, cnameTarget, nil
		}
		if hasType(response.Answer, qname, qtype) {
			hop.ResponseKind = "answer"
			hop.Explanation = "The server returned the requested records."
			hop.TechnicalNote = "The answer section contains records that match the requested name and type."
			return hop, response, nil, "", nil
		}
		if isNODATA(response) {
			hop.ResponseKind = "nodata"
			hop.Explanation = "The server answered successfully, but no records of the requested type were present."
			hop.TechnicalNote = "NOERROR with an authoritative negative answer and no matching data is treated as NODATA."
			return hop, response, nil, "", nil
		}
		referralCandidates, nextTargets, note, err := e.nextCandidatesForReferral(ctx, state, response, hop.Index)
		if err == nil && len(referralCandidates) > 0 {
			hop.ResponseKind = "referral"
			hop.NextTargets = nextTargets
			hop.Explanation = "The server referred the trace to the next set of nameservers."
			hop.TechnicalNote = note
			return hop, response, referralCandidates, "", nil
		}
		hop.ResponseKind = "error"
		hop.Explanation = "The response did not contain a usable answer or referral."
		hop.TechnicalNote = "The trace could not derive a safe next target from the response."
		return hop, response, nil, "", errUnusableReferral
	}
	if timeoutCount == len(candidates) && len(candidates) > 0 {
		return lastHop, nil, nil, "", errTimeout
	}
	if lastErr == nil {
		lastErr = errUnusableReferral
	}
	return lastHop, nil, nil, "", lastErr
}

func (e *Engine) nextCandidatesForReferral(ctx context.Context, state *traceState, response *dns.Msg, parentIndex int) ([]ServerCandidate, []contracts.NextTarget, string, error) {
	nsNames := nsTargets(response.Ns)
	if len(nsNames) == 0 {
		return nil, nil, "", errUnusableReferral
	}
	glue := glueAddrs(response.Extra)
	zone := referralZone(response.Ns)
	candidates := make([]ServerCandidate, 0, len(nsNames))
	nextTargets := make([]contracts.NextTarget, 0, len(nsNames))
	for _, nsName := range nsNames {
		if ips, ok := glue[nsName]; ok {
			for _, ip := range ips {
				if err := e.cfg.DestinationPolicy.Allow(ip); err != nil {
					continue
				}
				candidates = append(candidates, ServerCandidate{name: nsName, ip: ip, endpoint: e.cfg.EndpointResolver(ip), zone: zone})
				nextTargets = append(nextTargets, contracts.NextTarget{ServerName: nsName, ServerIP: ip, ZoneContext: zone, Reason: "Glue from referral response."})
			}
			continue
		}
		ips, err := e.resolveNameServerAddresses(ctx, state, nsName, parentIndex)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if err := e.cfg.DestinationPolicy.Allow(ip); err != nil {
				continue
			}
			candidates = append(candidates, ServerCandidate{name: nsName, ip: ip, endpoint: e.cfg.EndpointResolver(ip), zone: zone})
			nextTargets = append(nextTargets, contracts.NextTarget{ServerName: nsName, ServerIP: ip, ZoneContext: zone, Reason: "Resolved nameserver address with a support lookup."})
		}
	}
	if len(candidates) == 0 {
		return nil, nil, "", errUnusableReferral
	}
	return candidates, nextTargets, "Referral continuation used glue when available and support lookups when glue was missing.", nil
}

func (e *Engine) resolveNameServerAddresses(ctx context.Context, state *traceState, nsName string, parentIndex int) ([]string, error) {
	for _, qtype := range []string{"A", "AAAA"} {
		parent := parentIndex
		outcome := e.traceName(ctx, state, fqdn(strings.TrimSuffix(nsName, ".")), qtype, e.cfg.Roots, &parent, "nameserver_address_lookup", ".")
		if outcome.Kind != "success" {
			continue
		}
		var ips []string
		for _, hop := range state.hops {
			if hop.ParentIndex != nil && *hop.ParentIndex == parentIndex && hop.HopPurpose == "nameserver_address_lookup" && hop.ResponseKind == "answer" && hop.QName == nsName {
				for _, rrset := range hop.AnswerRRSets {
					if rrset.Type == qtype {
						ips = append(ips, rrset.Data...)
					}
				}
			}
		}
		if len(ips) > 0 {
			return uniqueStrings(ips), nil
		}
	}
	return nil, errUnusableReferral
}

func (e *Engine) exchange(ctx context.Context, endpoint, qname, qtype string) (*dns.Msg, time.Duration, string, bool, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(qname, dns.StringToType[qtype])
	msg.RecursionDesired = false

	udpClient := &dns.Client{Net: "udp", Timeout: e.cfg.PerHopTimeout}
	start := time.Now()
	resp, _, err := udpClient.ExchangeContext(ctx, msg, endpoint)
	latency := time.Since(start)
	if err != nil {
		return nil, 0, "", false, err
	}
	if resp.Truncated {
		tcpClient := &dns.Client{Net: "tcp", Timeout: e.cfg.PerHopTimeout}
		start = time.Now()
		resp, _, err = tcpClient.ExchangeContext(ctx, msg, endpoint)
		latency += time.Since(start)
		if err != nil {
			return nil, 0, "", true, err
		}
		return resp, latency, "tcp", true, nil
	}
	return resp, latency, "udp", false, nil
}

func rrsetsFromSection(section string, records []dns.RR) []contracts.RRSet {
	grouped := map[string]*contracts.RRSet{}
	order := make([]string, 0)
	for _, record := range records {
		hdr := record.Header()
		key := fmt.Sprintf("%s|%s", hdr.Name, dns.TypeToString[hdr.Rrtype])
		rrset, ok := grouped[key]
		if !ok {
			rrset = &contracts.RRSet{
				Section: section,
				Name:    hdr.Name,
				Type:    dns.TypeToString[hdr.Rrtype],
				TTL:     hdr.Ttl,
			}
			grouped[key] = rrset
			order = append(order, key)
		}
		rrset.Data = append(rrset.Data, recordString(record))
	}
	result := make([]contracts.RRSet, 0, len(order))
	for _, key := range order {
		result = append(result, *grouped[key])
	}
	return result
}

func recordString(record dns.RR) string {
	switch rr := record.(type) {
	case *dns.A:
		return rr.A.String()
	case *dns.AAAA:
		return rr.AAAA.String()
	case *dns.NS:
		return rr.Ns
	case *dns.CNAME:
		return rr.Target
	case *dns.SOA:
		return fmt.Sprintf("%s %s %d %d %d %d %d", rr.Ns, rr.Mbox, rr.Serial, rr.Refresh, rr.Retry, rr.Expire, rr.Minttl)
	default:
		return rr.String()
	}
}

func hasType(records []dns.RR, qname, qtype string) bool {
	for _, record := range records {
		if strings.EqualFold(record.Header().Name, qname) && dns.TypeToString[record.Header().Rrtype] == qtype {
			return true
		}
	}
	return false
}

func firstCNAME(records []dns.RR, qname string) string {
	for _, record := range records {
		cname, ok := record.(*dns.CNAME)
		if ok && strings.EqualFold(cname.Hdr.Name, qname) {
			return cname.Target
		}
	}
	return ""
}

func isNODATA(response *dns.Msg) bool {
	if response.Rcode != dns.RcodeSuccess || !response.Authoritative || len(response.Answer) > 0 {
		return false
	}
	for _, record := range response.Ns {
		if record.Header().Rrtype == dns.TypeSOA {
			return true
		}
	}
	return false
}

func nsTargets(records []dns.RR) []string {
	var targets []string
	for _, record := range records {
		ns, ok := record.(*dns.NS)
		if ok {
			targets = append(targets, ns.Ns)
		}
	}
	return uniqueStrings(targets)
}

func glueAddrs(records []dns.RR) map[string][]string {
	result := map[string][]string{}
	for _, record := range records {
		switch rr := record.(type) {
		case *dns.A:
			result[rr.Hdr.Name] = append(result[rr.Hdr.Name], rr.A.String())
		case *dns.AAAA:
			result[rr.Hdr.Name] = append(result[rr.Hdr.Name], rr.AAAA.String())
		}
	}
	return result
}

func referralZone(records []dns.RR) string {
	for _, record := range records {
		if ns, ok := record.(*dns.NS); ok {
			return ns.Hdr.Name
		}
	}
	return "."
}

func classifyRole(zone, purpose string, response *dns.Msg, qname, qtype string) string {
	if purpose == "nameserver_address_lookup" {
		return "authoritative"
	}
	if purpose == "cname_follow" && firstCNAME(response.Answer, qname) != "" {
		return "alias"
	}
	if hasType(response.Answer, qname, qtype) || isNODATA(response) || response.Rcode != dns.RcodeSuccess {
		return "final"
	}
	if zone == "." {
		return "root"
	}
	if countLabels(zone) == 1 {
		return "tld"
	}
	return "authoritative"
}

func explainRCode(rcode int) string {
	switch rcode {
	case dns.RcodeNameError:
		return "The authoritative server reported that the name does not exist."
	case dns.RcodeServerFailure:
		return "The upstream server reported a server failure."
	case dns.RcodeRefused:
		return "The upstream server refused the query."
	case dns.RcodeNotImplemented:
		return "The upstream server does not implement the requested operation."
	default:
		return "The upstream server returned a non-success response code."
	}
}

func newErrorHop(index int, parent *int, candidate ServerCandidate, qname, qtype, purpose, zone, rcode, responseKind, explanation, technicalNote string) contracts.Hop {
	return contracts.Hop{
		Index:            index,
		ParentIndex:      parent,
		Role:             "error",
		HopPurpose:       purpose,
		ZoneContext:      zone,
		ServerName:       candidate.name,
		ServerIP:         candidate.ip,
		QName:            qname,
		QType:            qtype,
		Transport:        "udp",
		LatencyMS:        0,
		ResponseCode:     rcode,
		Authoritative:    false,
		Truncated:        false,
		ResponseKind:     responseKind,
		AnswerRRSets:     []contracts.RRSet{},
		AuthorityRRSets:  []contracts.RRSet{},
		AdditionalRRSets: []contracts.RRSet{},
		NextTargets:      []contracts.NextTarget{},
		Explanation:      explanation,
		TechnicalNote:    technicalNote,
	}
}

func normalizeHop(hop contracts.Hop) contracts.Hop {
	if hop.AnswerRRSets == nil {
		hop.AnswerRRSets = []contracts.RRSet{}
	}
	if hop.AuthorityRRSets == nil {
		hop.AuthorityRRSets = []contracts.RRSet{}
	}
	if hop.AdditionalRRSets == nil {
		hop.AdditionalRRSets = []contracts.RRSet{}
	}
	if hop.NextTargets == nil {
		hop.NextTargets = []contracts.NextTarget{}
	}
	return hop
}

func summaryDetail(kind, domain, qtype string) string {
	switch kind {
	case "success":
		return fmt.Sprintf("The trace completed for %s %s.", domain, qtype)
	case "nxdomain":
		return "The trace reached an authoritative NXDOMAIN response."
	case "timeout":
		return "The backend did not receive a timely upstream response."
	case "unusable_referral":
		return "The trace received a referral but could not continue safely."
	default:
		return "The trace ended with a terminal failure."
	}
}

func newTraceID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func fqdn(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

func lastHopIndex(hops []contracts.Hop) int {
	if len(hops) == 0 {
		return -1
	}
	return hops[len(hops)-1].Index
}

func countLabels(zone string) int {
	trimmed := strings.TrimSuffix(zone, ".")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "."))
}

func RootCandidatesForTests(candidates []ServerCandidate) []ServerCandidate {
	return slices.Clone(candidates)
}
