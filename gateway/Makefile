.PHONY: all build push
TAG?=0.20.1
NS?=openfaas

all:	build
build:
	./build.sh ${TAG} ${NS}
push:
	./push.sh ${TAG} ${NS}
