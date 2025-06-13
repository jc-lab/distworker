package config

// envMapping for multi-word environment variable names
var envMapping = map[string]string{
	"SERVER_API_OPENAI_CHAT_COMPLETIONS_QUEUE": "server.api.openai.chat_completions_queue",
	"SERVER_API_OPENAI_COMPLETIONS_QUEUE":      "server.api.openai.completions_queue",
	"SERVER_API_OPENAI_EMBEDDINGS_QUEUE":       "server.api.openai.embeddings_queue",
	"SERVER_WORKER_ACCESSIBLE_BASE_URL":        "server.worker.accessible_base_url",
}

func EnvNameToKey(key string) string {
	return envMapping[key]
}
