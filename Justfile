set shell := ["bash", "-uc"]

alias b := build
alias r := run
alias rd := run-dev
alias t := test
alias ts := test-short
alias tb := bench
alias l := lint
alias f := fmt
alias fc := fmt-check
alias c := ci
alias k := docker-stop

default:
	@just help

help:
	@make help

build:
	@make build

run:
	@make run

run-dev:
	@make run-dev

test:
	@make test

test-short:
	@make test-short

bench:
	@make bench


lint:
	@make lint

fmt:
	@make fmt

fmt-check:
	@make fmt-check

tidy:
	@make tidy

vet:
	@make vet

ci:
	@make ci

clean:
	@make clean

clean-cache:
	@make clean-cache

clean-all:
	@make clean-all

docker-build:
	@make docker-build

docker-run:
	@make docker-run

docker-stop:
	@make docker-stop

docker-shell:
	@make docker-shell

bootstrap \
  host="127.0.0.1" \
  port="6379" \
  node="node-1" \
  raft="127.0.0.1:7000" \
  http="127.0.0.1:17000":
	@make bootstrap \
	  BOOTSTRAP_HOST={{host}} \
	  BOOTSTRAP_PORT={{port}} \
	  BOOTSTRAP_NODE={{node}} \
	  BOOTSTRAP_RAFT={{raft}} \
	  BOOTSTRAP_HTTP={{http}}

join \
  node \
  port \
  raft \
  http \
  leader:
	@make join \
	  JOIN_NODE={{node}} \
	  JOIN_PORT={{port}} \
	  JOIN_RAFT={{raft}} \
	  JOIN_HTTP={{http}} \
	  JOIN_TARGET={{leader}}

rejoin node host port raft http:
	@make rejoin \
	  REJOIN_NODE={{node}} \
	  REJOIN_PORT={{port}} \
	  REJOIN_RAFT={{raft}} \
	  REJOIN_HTTP={{http}} \
	  REJOIN_HOST={{host}}

cluster-up:
	@make cluster-up

cluster-down:
	@make cluster-down
