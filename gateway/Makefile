.PHONY: all build push
TAG?=0.19.6
NS?=openfaas

all:	build
build:
	./build.sh ${TAG} ${NS}
push:
	./push.sh ${TAG} ${NS}
