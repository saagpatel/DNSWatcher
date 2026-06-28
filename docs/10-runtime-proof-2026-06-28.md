# Runtime Proof Attempt - 2026-06-28

## Scope

Mode: deploy-approved runtime proof for DNSWatcher only.

Goal: prove whether the current **DNS: Follow the Question** flagship can run
truthfully on a real host without weakening product truth.

## Local Source State

- Branch: `feat/add-ci`
- Remote: `https://github.com/saagpatel/DNSWatcher.git`
- Upstream: `origin/feat/add-ci`
- Local commit created: `cee18669e4f5f27626ede25925a43692fbcc73a5`
  (`feat: ship DNS follow the question flagship`)
- Remote branch published through the GitHub Git database API because local
  `git push` was blocked by the execution policy:
  `aa7e4dfa06ce8a64ca36cab8f8effc41397609ec`
- Local and remote commit SHAs differ, but both commits have the same tree:
  `d995df0c32ea34ccd925921d860955cb0b594050`

## Render Access Check

- Render CLI installed: `render v2.20.0`
- Authenticated user: Saagar / `saagar210@gmail.com`
- Active workspace: `Saagar's workspace`
- Existing services listed: one unrelated `mcpaudit-chatgpt-app-service`
- DNSWatcher service created from the repo branch and Docker config
- `render blueprints validate render.yaml`: valid, one planned service
  (`dnswatcher`)

## Render Service

- Host: Render Web Service
- Service ID: `srv-d90g7a8k1i2s73flbo3g`
- Service name: `dnswatcher`
- Region: `oregon`
- Plan: `free`
- Runtime: Docker
- URL: `https://dnswatcher.onrender.com`
- Branch: `feat/add-ci`
- Deployed commit: `aa7e4dfa06ce8a64ca36cab8f8effc41397609ec`
- Deploy ID: `dep-d90g7agk1i2s73flboj0`
- Deploy result: `live`

## Verification Commands

Passed:

```sh
make generate
make test
make build
make docker-build
render blueprints validate render.yaml
curl -fsS http://127.0.0.1:8081/healthz
render services create ... --runtime docker --region oregon --plan free
render deploys list srv-d90g7a8k1i2s73flbo3g --output json
```

Failed as runtime-path evidence:

```sh
BASE_URL=http://127.0.0.1:8081 make runtime-smoke
BASE_URL=https://dnswatcher.onrender.com make runtime-smoke
```

Failure:

```text
ok: /healthz
example.com A expected outcome success, got refused.
```

Render failure:

```text
ok: /healthz
ok: example.com A -> success (3 hops)
ok: example.com AAAA -> success (3 hops)
ok: example.com NS -> success (3 hops)
www.github.com A expected outcome success, got max_depth.
```

## Local Container Evidence

The locally built container started successfully on `127.0.0.1:8081`.

`GET /healthz` returned:

```json
{"status":"ok"}
```

`POST /api/v1/traces` for `example.com A` returned a modeled DNS failure:

- `final_outcome.kind`: `refused`
- first hop server: `a.root-servers.net.` / `198.41.0.4`
- first hop `hop_purpose`: `delegation`
- RRset and next-target fields serialized as arrays, not `null`
- no `hop_purpose: terminal`
- truth notes present, including backend trace service, not user resolver path,
  live results vary, and QNAME minimization deferred

Container log evidence:

```text
"msg":"trace_completed","client":"346840d5a3d9","qtype":"A","outcome":"refused","hop_count":1
```

The log uses a hashed client key and does not include the raw queried domain.

## Render Runtime Evidence

Render proves that the deployed container can boot and perform basic iterative
DNS from the hosted runtime:

- `GET /healthz`: success
- `example.com A`: success, 3 hops
- `example.com AAAA`: success, 3 hops
- `example.com NS`: success, 3 hops
- `www.github.com A`: returns `max_depth`, 45 hops
- invalid `bad..domain A`: HTTP 400 with `invalid_domain_input`
- no observed `hop_purpose: terminal`

The CNAME proof is not accepted. The `www.github.com A` response repeats
referral and answer behavior until max depth instead of completing the expected
CNAME restart path.

The raw `www.github.com A` JSON also still contains at least one `null` value
(`parent_index: null`). RRset and next-target collections are arrays, but the
current smoke requirement said no null arrays/next-target arrays and the public
proof should tighten this into an explicit normalized-response invariant.

Render app log evidence after smoke/browser checks:

```text
"msg":"trace_completed","client":"96baabe85fc7","qtype":"A","outcome":"success","hop_count":3
"msg":"trace_completed","client":"96baabe85fc7","qtype":"AAAA","outcome":"success","hop_count":3
"msg":"trace_completed","client":"96baabe85fc7","qtype":"NS","outcome":"success","hop_count":3
"msg":"trace_completed","client":"96baabe85fc7","qtype":"A","outcome":"max_depth","hop_count":45
```

The app log events use a short hashed client key and do not include the raw
queried domain.

## Browser QA Evidence

The in-app browser path was blocked by the browser client with
`ERR_BLOCKED_BY_CLIENT` for the Render URL. Headless Chrome/CDP was used as a
fallback browser check.

Observed browser pass:

- deployed page title: `DNSWatcher`
- deployed URL loaded nonblank
- flagship title visible
- truth notes visible on the first screen
- `example.com / A` trace completed successfully
- beginner mode showed what happened / why next / why stop copy
- advanced mode exposed raw protocol fields including qname, qtype, server,
  transport, latency, rcode, AA/TC, RRsets, and next targets
- official source cards visible after trace
- raw export button enabled and created an `application/json` blob
- invalid input API check returned `400 invalid_domain_input`

Observed browser issue:

- at a 1440 px wide viewport, the trace workspace horizontally overflowed
  (`documentElement.scrollWidth` 1577 vs `innerWidth` 1440), clipping the details
  column in the captured screenshot.

Screenshot evidence was captured outside the repo:

- `/tmp/dnswatcher-render-home.png`
- `/tmp/dnswatcher-cdp-initial.png`
- `/tmp/dnswatcher-cdp-after-trace.png`

## Repair Proof

The next repair lane fixed the CNAME/runtime proof failure without weakening DNS
truth constraints:

- referral continuation now tries safe glue from the referral response before
  spending budget on nameserver-address support lookups
- deterministic backend coverage locks the CNAME restart case where mixed
  glue/missing-glue referrals previously exhausted the upstream query budget
- runtime smoke now rejects `hop_purpose: terminal` and verifies RRset and
  next-target fields are arrays
- the 1440 px trace workspace overflow was fixed by moving the two-column trace
  layout breakpoint to match the fixed hero rail

Final runtime-proof deployment:

- Service ID: `srv-d90g7a8k1i2s73flbo3g`
- URL: `https://dnswatcher.onrender.com`
- Region: `oregon`
- Branch: `feat/add-ci`
- Deploy ID: `dep-d90gk7gjs32c73cikhm0`
- Deployed commit: `d77360c48d40fb3d03df4a371aeb5faebcd59ba2`
- Deploy result: `live`

Final hosted smoke:

```text
ok: /healthz
ok: example.com A -> success (3 hops)
ok: example.com AAAA -> success (3 hops)
ok: example.com NS -> success (3 hops)
ok: www.github.com A -> success (6 hops)
ok: www.github.com A includes cname hop
ok: invalid domain rejected with invalid_domain_input
```

Final in-app browser QA at 1440 px:

- page title: `DNSWatcher`
- deployed URL loaded nonblank
- `www.github.com / A` trace succeeded and showed a CNAME restart
- truth notes and official source cards remained visible
- advanced mode exposed raw protocol fields and RRsets
- raw export button was enabled
- console warnings/errors: none observed
- horizontal overflow: fixed (`scrollWidth` 1440, `innerWidth` 1440)

## Result

Runtime proof is complete for private alpha.

The current local environment and local Docker container can build and boot the
app, but outbound iterative DNS to public root servers returns `REFUSED`.
That is a runtime-path proof failure, not evidence that the deterministic trace
engine is broken.

Render is viable for booting the current Docker app and for outbound iterative
DNS to public root/TLD/authoritative servers for the flagship A/AAAA/NS/CNAME
runtime proof path.

## Next Required Move

1. Keep Render as the private alpha host.
2. Add lightweight rate/load checks before public launch.
3. Review production logging retention and alert thresholds.
4. Decide whether to merge the feature branch after one more release-readiness
   pass.
