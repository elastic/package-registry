TAG := $(shell date +'%Y-%m-%d')-$(shell git log --pretty=format:%h -1)
REPO := package-registry:$(TAG)
DOCKER_PATH := push.docker.elastic.co/employees/ruflin/$(REPO)

build:
	docker build -t $(REPO) .

publish: build
	docker tag $(REPO) $(DOCKER_PATH)
	docker push $(DOCKER_PATH)
