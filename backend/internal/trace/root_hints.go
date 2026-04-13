package trace

func DefaultRootCandidates() []ServerCandidate {
	return []ServerCandidate{
		{name: "a.root-servers.net.", ip: "198.41.0.4", endpoint: "198.41.0.4:53", zone: "."},
		{name: "b.root-servers.net.", ip: "170.247.170.2", endpoint: "170.247.170.2:53", zone: "."},
		{name: "c.root-servers.net.", ip: "192.33.4.12", endpoint: "192.33.4.12:53", zone: "."},
		{name: "d.root-servers.net.", ip: "199.7.91.13", endpoint: "199.7.91.13:53", zone: "."},
		{name: "e.root-servers.net.", ip: "192.203.230.10", endpoint: "192.203.230.10:53", zone: "."},
		{name: "f.root-servers.net.", ip: "192.5.5.241", endpoint: "192.5.5.241:53", zone: "."},
		{name: "g.root-servers.net.", ip: "192.112.36.4", endpoint: "192.112.36.4:53", zone: "."},
		{name: "h.root-servers.net.", ip: "198.97.190.53", endpoint: "198.97.190.53:53", zone: "."},
		{name: "i.root-servers.net.", ip: "192.36.148.17", endpoint: "192.36.148.17:53", zone: "."},
		{name: "j.root-servers.net.", ip: "192.58.128.30", endpoint: "192.58.128.30:53", zone: "."},
		{name: "k.root-servers.net.", ip: "193.0.14.129", endpoint: "193.0.14.129:53", zone: "."},
		{name: "l.root-servers.net.", ip: "199.7.83.42", endpoint: "199.7.83.42:53", zone: "."},
		{name: "m.root-servers.net.", ip: "202.12.27.33", endpoint: "202.12.27.33:53", zone: "."},
	}
}
