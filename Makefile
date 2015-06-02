APP_NAME=htflood

DOCKER_IMG_NAME=$(APP_NAME)-bot
DOCKER_IMG_PUB_NAME=vincentcr/$(DOCKER_IMG_NAME)
DOCKER_TEST_CONTAINER ?= $(APP_NAME)-bot
API_KEY     ?= 212af9729e854cb3b4d2715978527575
PORT        ?= 3210
DOCKER_PORT ?= 3211
CERT_HOSTNAME ?= localhost
TARGETS=$(APP_NAME)
SOURCE_FILES=$(shell find . -type f -name '*.go')

build: $(APP_NAME)

rebuild:
	$(MAKE) clean
	$(MAKE) build

$(APP_NAME): $(SOURCE_FILES)
	go build .

tls-cert:
	@if [ -f tls-cert.pem ] ; then \
		echo cert already exists; \
		false ;\
	else  \
		openssl req -nodes -x509 -newkey rsa:2048 -keyout tls-key.pem -out tls-cert.pem -days 720 -subj "/CN=$(CERT_HOSTNAME)" ;\
	fi ;\

docker-publish: docker-build
	docker tag -f $(DOCKER_IMG_NAME) $(DOCKER_IMG_PUB_NAME)
	docker push $(DOCKER_IMG_PUB_NAME)

docker-build:
	docker build -t $(DOCKER_IMG_NAME) .

example-get: build
	./$(APP_NAME) -count 50 -concurrency 10 http://www.yahoo.com/

example-post: build
	./$(APP_NAME) -count 1 -concurrency 1 \
	POST https://www.yahoo.com/ \
	Content-Type:application/json \
	foo=bar x:=1

example-scenario: build
	./$(APP_NAME) < ./examples/example.json

example-with-local-bot: build
	./$(APP_NAME) -bots http://localhost:$(PORT) -api-key $(API_KEY) -count 50 -concurrency 10 http://www.yahoo.com/

UNAME := $(shell uname)
ifeq ($(UNAME), Darwin) # mac
	DOCKER_BOT_HOST=$(shell boot2docker ip)
else
	DOCKER_BOT_HOST=localhost
endif

example-with-local-docker-bot: build
	./$(APP_NAME) -bots http://$(DOCKER_BOT_HOST):$(DOCKER_PORT) -api-key $(API_KEY) -count 50 -concurrency 10 http://www.yahoo.com/

run-bot: build
	./$(APP_NAME) bot -api-key="$(API_KEY)" -tls-cert=tls-cert.pem -tls-key=tls-key.pem

docker-run: docker-build
	docker run -ti --rm -e PORT=$(DOCKER_PORT) -e API_KEY=$(API_KEY) \
		--name $(DOCKER_TEST_CONTAINER) \
		--publish $(DOCKER_PORT):$(DOCKER_PORT) \
		$(DOCKER_IMG_NAME)

docker-rm:
	docker stop $(DOCKER_TEST_CONTAINER)
	docker rm $(DOCKER_TEST_CONTAINER)


clean:
	rm -fr $(TARGETS)
