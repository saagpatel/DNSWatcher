package contracts

type TraceRequest struct {
	Domain string `json:"domain"`
	QType  string `json:"qtype"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type TraceResult struct {
	TraceID         string       `json:"trace_id"`
	InputDomain     string       `json:"input_domain"`
	NormalizedDomain string      `json:"normalized_domain"`
	QType           string       `json:"qtype"`
	StartedAt       string       `json:"started_at"`
	CompletedAt     string       `json:"completed_at"`
	TotalDurationMS int          `json:"total_duration_ms"`
	Status          string       `json:"status"`
	FinalOutcome    FinalOutcome `json:"final_outcome"`
	Hops            []Hop        `json:"hops"`
	Summary         TraceSummary `json:"summary"`
	TruthNotes      []TruthNote  `json:"truth_notes"`
}

type TraceSummary struct {
	Headline    string `json:"headline"`
	Detail      string `json:"detail"`
	TotalHops   int    `json:"total_hops"`
	AnswerCount int    `json:"answer_count"`
	CNAMECount  int    `json:"cname_count"`
}

type FinalOutcome struct {
	Kind             string `json:"kind"`
	RCode            string `json:"rcode"`
	Message          string `json:"message"`
	TerminalHopIndex int    `json:"terminal_hop_index"`
}

type TruthNote struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Hop struct {
	Index          int          `json:"index"`
	ParentIndex    *int         `json:"parent_index"`
	Role           string       `json:"role"`
	HopPurpose     string       `json:"hop_purpose"`
	ZoneContext    string       `json:"zone_context"`
	ServerName     string       `json:"server_name"`
	ServerIP       string       `json:"server_ip"`
	QName          string       `json:"qname"`
	QType          string       `json:"qtype"`
	Transport      string       `json:"transport"`
	LatencyMS      int          `json:"latency_ms"`
	ResponseCode   string       `json:"response_code"`
	Authoritative  bool         `json:"authoritative"`
	Truncated      bool         `json:"truncated"`
	ResponseKind   string       `json:"response_kind"`
	AnswerRRSets   []RRSet      `json:"answer_rrsets"`
	AuthorityRRSets []RRSet     `json:"authority_rrsets"`
	AdditionalRRSets []RRSet    `json:"additional_rrsets"`
	NextTargets    []NextTarget `json:"next_targets"`
	Explanation    string       `json:"explanation"`
	TechnicalNote  string       `json:"technical_note"`
}

type NextTarget struct {
	ServerName string `json:"server_name"`
	ServerIP   string `json:"server_ip"`
	ZoneContext string `json:"zone_context"`
	Reason     string `json:"reason"`
}

type RRSet struct {
	Section string   `json:"section"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     uint32   `json:"ttl"`
	Data    []string `json:"data"`
}
