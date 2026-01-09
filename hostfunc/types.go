package hostfunc

// KV store types

type KVGetRequest struct {
	Key string `json:"key"`
}

type KVSetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type KVDeleteRequest struct {
	Key string `json:"key"`
}

// HTTP types

type HTTPGetRequest struct {
	URL string `json:"url"`
}

type HTTPResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Filesystem types

type FSReadRequest struct {
	Path string `json:"path"`
}

type FSWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type FSListRequest struct {
	Path string `json:"path"`
}

type FSEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type FSExistsRequest struct {
	Path string `json:"path"`
}

type FSMkdirRequest struct {
	Path string `json:"path"`
}

type FSRemoveRequest struct {
	Path string `json:"path"`
}

type FSStatRequest struct {
	Path string `json:"path"`
}

type FSStatResponse struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}
