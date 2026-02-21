IMG ?= reflow/gateway-operator:latest

.PHONY: build
build:
	go build -o bin/manager cmd/main.go

.PHONY: run
run:
	go run cmd/main.go

.PHONY: test
test:
	go test ./... -coverprofile cover.out

.PHONY: install
install:
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall:
	kubectl delete -f config/crd/bases/

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push:
	docker push $(IMG)

.PHONY: deploy
deploy: install
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/

.PHONY: helm-install
helm-install:
	helm upgrade --install reflow-operator ./chart -n reflow-system --create-namespace

.PHONY: helm-uninstall
helm-uninstall:
	helm uninstall reflow-operator -n reflow-system

.PHONY: helm-template
helm-template:
	helm template reflow-operator ./chart
