build:
	docker build . -t go-arbitrager

run:
    docker-compose up -d

stop:
    docker-compose stop