.PHONY: run stop test build

build:
	docker compose build

run:
	docker compose up --build -d

stop:
	docker compose down

test:
	pytest tests3right.py -v