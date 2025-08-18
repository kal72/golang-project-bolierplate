package model

type Response struct {
	Status   string      `json:"status"`
	Data     interface{} `json:"data"`
	Message  string      `json:"message"`
	Metadata interface{} `json:"metadata,omitempty"`
}

type PageMetadata struct {
	Page      int   `json:"page"`
	Size      int   `json:"size"`
	TotalItem int64 `json:"total_item"`
	TotalPage int   `json:"total_page"`
}
