package models

type Pack struct {
	Images map[string]PackImage `json:"images"`
	Pack   PackInfo             `json:"pack"`
}

type PackInfo struct {
	DisplayName string   `json:"display_name"`
	AvatarUrl   *string  `json:"avatar_url,omitempty"`
	Attribution *string  `json:"attribution,omitempty"`
	Usage       []string `json:"usage,omitempty"`
}

type PackImage struct {
	Url   string         `json:"url"`
	Info  *PackImageInfo `json:"info,omitempty"`
	Usage []string       `json:"usage,omitempty"`
}

type PackImageInfo struct {
	MimeType string `json:"mimetype"`
	Size     *int   `json:"size,omitempty"`
	Width    *int   `json:"w,omitempty"`
	Height   *int   `json:"h,omitempty"`
}

type MessageResponse struct {
	Chunk []ClientEvent `json:"chunk"`
	End   string        `json:"end,omitempty"`
	Start string        `json:"start,omitempty"`
}

type ClientEvent struct {
	RoomId    string         `json:"room_id"`
	EventId   string         `json:"event_id"`
	EventType string         `json:"type"`
	Content   map[string]any `json:"content,omitempty"`
}
