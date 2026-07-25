COMPOSE := docker compose -f infra/compose/compose.yaml
API_READY_URL ?= http://127.0.0.1:8080/readyz

.PHONY: compose-up compose-start compose-down inspect-reports

compose-up:
	$(COMPOSE) up --build -d

compose-start: compose-up
	@until curl --fail --silent "$(API_READY_URL)" >/dev/null; do sleep 1; done

compose-down:
	$(COMPOSE) down --volumes

inspect-reports:
	$(COMPOSE) run --rm --no-deps api inspect-reports
