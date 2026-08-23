package workflows

import ()

type SearchClient struct {
	Url string
}

func NewSearchClient(url string) *SearchClient {
	return &SearchClient{
		Url: url,
	}
}
