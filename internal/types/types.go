package types

type CacheDomain struct {
	Name        string   `json:"name"`
	DomainFiles []string `json:"domain_files"`
}

type CacheDomainsResponse struct {
	CacheDomains []CacheDomain `json:"cache_domains"`
}

type DNSRewrite struct {
	Domain string `json:"domain"`
	Answer string `json:"answer"`
}