param(
  [string]$Url = "http://localhost:8080/v1/chat/completions",
  [string]$ApiKey = "",
  [string]$Model = "gpt-4",
  [string]$Prompt = "hello from loadtest",
  [int]$Requests = 200,
  [int]$Concurrency = 20,
  [string]$Timeout = "10s"
)

go run ./scripts/loadtest `
  -url $Url `
  -api-key $ApiKey `
  -model $Model `
  -prompt $Prompt `
  -requests $Requests `
  -concurrency $Concurrency `
  -timeout $Timeout
