docker:
	go build
	docker build -t haproxy-docker-wrapper .
