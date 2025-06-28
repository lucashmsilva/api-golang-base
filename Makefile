.PHONY: setup build up down start stop restart watch logs login test coverage audit migration
setup:
	echo "#TODO: setup script"
build:
	cd docker && docker compose build
up: setup
	cd docker && docker compose up -d
down: setup
	cd docker && docker compose down
start: setup
	cd docker && docker compose start
stop:
	cd docker && docker compose stop
restart: down up
watch: setup
	cd docker && export WATCH_FILES=1 && docker compose up -d
logs:
	cd docker && docker compose logs --tail=10 -f
login:
	docker exec -it api-golang-base-server sh

test:
	docker exec -it api-golang-base-server sh -c "./scripts/test.sh"
coverage:
	docker exec -it api-golang-base-server sh -c "./scripts/test.sh -c"
audit:
	docker exec -it api-golang-base-server sh -c "./scripts/audit.sh"
migration:
	echo "#TODO: migration script"
