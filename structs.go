package yadloader

type entryType string

const (
	FILE entryType = "file"
	DIR  entryType = "dir"
)

type response struct {
	Path       string    `json:"path"`
	Type       entryType `json:"type"`
	Name       string    `json:"name"`
	Created    string    `json:"created"`
	Modified   string    `json:"modified"`
	Size       *int64    `json:"size"`
	MD5        *string   `json:"md5"`
	SHA256     *string   `json:"sha256"`
	PublicKey  string    `json:"public_key"`
	MediaType  *string   `json:"media_type"`
	ResourceId string    `json:"resource_id"`
	File       *string   `json:"file"`
	Embedded   *embedded `json:"_embedded"`
}

type embedded struct {
	Path   string     `json:"path"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
	Sort   string     `json:"sort"`
	Total  int        `json:"total"`
	Items  []response `json:"items"`
}

type diskFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	File     string `json:"file"`
	MD5      string `json:"md5"`
	SHA256   string `json:"sha256"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}
