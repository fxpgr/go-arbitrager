build:
	docker build . -t go-arbitrager

run:
	docker run -it --rm --name arbitrager go-arbitrager go-arbitrager

run-cui:
	docker run -it --rm --name arbitrager go-arbitrager go-arbitrager -c config.yml -m cui
