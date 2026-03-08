package longbridge

import "encoding/json"

type StockNewsItem struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	URL           string `json:"url"`
	PublishedAt   string `json:"published_at"`
	CommentsCount int64  `json:"comments_count"`
	LikesCount    int64  `json:"likes_count"`
	SharesCount   int64  `json:"shares_count"`
}

type stockNewsResponse struct {
	Items []*StockNewsItem `json:"items"`
}

func (r *stockNewsResponse) UnmarshalJSON(data []byte) error {
	var direct []*StockNewsItem
	if err := json.Unmarshal(data, &direct); err == nil {
		r.Items = direct
		return nil
	}

	var wrapped struct {
		Items []*StockNewsItem `json:"items"`
		List  []*StockNewsItem `json:"list"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return err
	}
	if len(wrapped.Items) > 0 {
		r.Items = wrapped.Items
		return nil
	}
	r.Items = wrapped.List
	return nil
}
