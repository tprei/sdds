COMPOSE := docker compose -f infra/compose/compose.yaml
PREFLIGHT_COMPOSE := docker compose -f infra/compose/compose-preflight.yaml
API_READY_URL ?= http://127.0.0.1:8080/readyz

.PHONY: compose-up compose-start compose-down inspect-reports export-events reindex-embeddings

compose-up:
	COMPOSE_IGNORE_ORPHANS=1 $(PREFLIGHT_COMPOSE) run --build --rm --no-deps --entrypoint /usr/local/bin/validate-compose-secrets rustfs-init
	$(COMPOSE) up --build -d

compose-start: compose-up
	@until curl --fail --silent "$(API_READY_URL)" >/dev/null; do sleep 1; done

compose-down:
	$(COMPOSE) down --volumes

inspect-reports:
	$(COMPOSE) run --rm --no-deps api inspect-reports


export-events:
	$(COMPOSE) run --rm --no-deps api export-events

reindex-embeddings:
	$(COMPOSE) run --rm api reindex-embeddings