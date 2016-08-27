.PHONY: build deploy-app deploy-worker

build:
	go build -x ./...

deploy-app: build
	cd app && aedeploy gcloud app deploy

deploy-worker: build
	cd pubsub_worker && aedeploy gcloud app deploy


