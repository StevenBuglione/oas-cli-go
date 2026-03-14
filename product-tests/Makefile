.PHONY: check-prereqs services-up services-down smoke full

check-prereqs:
	@bash scripts/check-prereqs.sh

services-up: check-prereqs
	@bash scripts/services-up.sh

services-down:
	@bash scripts/services-down.sh

smoke: check-prereqs
	@echo "==> Validating compose configs..."
	docker compose config --quiet
	docker compose -f mcp/remote/docker-compose.yml config --quiet
	@echo "==> Smoke OK (infrastructure configs valid)"

full: services-up smoke
	@echo "==> Full suite placeholder — add test targets here"
	$(MAKE) services-down
