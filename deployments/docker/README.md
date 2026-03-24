# Local Integration Environment

This docker-compose stack starts:

- `gateway` (this project)
- `postgres`
- `redis`
- `otel-collector` (receives OTLP traces from gateway)
- `jaeger` (UI at `http://localhost:16686`)
- `mock-openai` (mock upstream LLM)
- `mock-embedding-worker` (mock embedding worker)

## Start

```bash
cd deployments/docker
docker compose up -d --build
```

## Verify

```bash
curl http://localhost:8080/health
curl http://localhost:8080/v1/models
```

Open Jaeger UI:

```text
http://localhost:16686
```

## Stop

```bash
docker compose down
```

## Notes

- Gateway uses `config.docker.yaml` in this folder.
- Traces flow: `gateway -> otel-collector -> jaeger`.
- Request and workflow logs are written under `./logs` in the project root.
