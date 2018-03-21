build:
	docker build . -t go-arbitrager

run:
	docker run -it --rm --name arbitrager go-arbitrager go-arbitrager config.yml
