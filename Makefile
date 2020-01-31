STACK_CONFIG ?= stack.jsonnet
SAM_CONFIG ?= sam.jsonnet

CODE_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
CWD := ${CURDIR}
BINPATH := $(CWD)/build/makePartition $(CWD)/build/listIndexObject $(CWD)/build/mergeIndexObject $(CWD)/build/apiHandler $(CWD)/build/errorHandler
SRC := $(CODE_DIR)/internal/*.go $(CODE_DIR)/pkg/*/*.go

TEMPLATE_FILE := template.json
SAM_FILE := sam.yml
BASE_FILE := $(CODE_DIR)/template.libsonnet
OUTPUT_FILE := $(CWD)/output.json

STACK_NAME := $(shell jsonnet $(STACK_CONFIG) | jq .StackName)
BUILD_OPT :=

ifdef TAGS
TAGOPT=--tags $(TAGS)
else
TAGOPT=
endif

all: $(OUTPUT_FILE)

clean:
	rm -f $(BINPATH)

vendor:
	cd $(CODE_DIR) && go mod vendor && cd $(CWD)

build: $(BINPATH)

testplugin:
	cd $(CODE_DIR) && go test -v ./internal && cd $(CWD)

$(CWD)/build/makePartition: $(CODE_DIR)/lambda/makePartition/*.go $(SRC)
	cd $(CODE_DIR) && env GOARCH=amd64 GOOS=linux go build -v $(BUILD_OPT) -o $(CWD)/build/makePartition $(CODE_DIR)/lambda/makePartition && cd $(CWD)
$(CWD)/build/listIndexObject: $(CODE_DIR)/lambda/listIndexObject/*.go $(SRC)
	cd $(CODE_DIR) && env GOARCH=amd64 GOOS=linux go build -v $(BUILD_OPT) -o $(CWD)/build/listIndexObject $(CODE_DIR)/lambda/listIndexObject && cd $(CWD)
$(CWD)/build/mergeIndexObject: $(CODE_DIR)/lambda/mergeIndexObject/*.go $(SRC)
	cd $(CODE_DIR) && env GOARCH=amd64 GOOS=linux go build -v $(BUILD_OPT) -o $(CWD)/build/mergeIndexObject $(CODE_DIR)/lambda/mergeIndexObject && cd $(CWD)
$(CWD)/build/apiHandler: $(CODE_DIR)/lambda/apiHandler/*.go $(SRC)
	cd $(CODE_DIR) && env GOARCH=amd64 GOOS=linux go build -v $(BUILD_OPT) -o $(CWD)/build/apiHandler $(CODE_DIR)/lambda/apiHandler && cd $(CWD)
$(CWD)/build/errorHandler: $(CODE_DIR)/lambda/errorHandler/*.go $(SRC)
	cd $(CODE_DIR) && env GOARCH=amd64 GOOS=linux go build -v $(BUILD_OPT) -o $(CWD)/build/errorHandler $(CODE_DIR)/lambda/errorHandler && cd $(CWD)

$(TEMPLATE_FILE): $(SAM_CONFIG) $(BASE_FILE)
	jsonnet -J $(CODE_DIR) $(SAM_CONFIG) -o $(TEMPLATE_FILE)

$(SAM_FILE): $(TEMPLATE_FILE) $(BINPATH) $(STACK_CONFIG)
	aws cloudformation package \
		--region $(shell jsonnet $(STACK_CONFIG) | jq .Region) \
		--template-file $(TEMPLATE_FILE) \
		--s3-bucket $(shell jsonnet $(STACK_CONFIG) | jq .CodeS3Bucket) \
		--s3-prefix $(shell jsonnet $(STACK_CONFIG) | jq .CodeS3Prefix) \
		--output-template-file $(SAM_FILE)

$(OUTPUT_FILE): $(SAM_FILE)
	aws cloudformation deploy \
		--region $(shell jsonnet $(STACK_CONFIG) | jq .Region) \
		--template-file $(SAM_FILE) \
		--stack-name $(STACK_NAME) \
		--capabilities CAPABILITY_IAM \
		$(TAGOPT) \
		--no-fail-on-empty-changeset
	aws cloudformation describe-stack-resources \
		--region $(shell jsonnet $(STACK_CONFIG) | jq .Region) \
		--stack-name $(STACK_NAME) > $(OUTPUT_FILE)


delete:
	aws cloudformation delete-stack \
		--region $(shell jsonnet $(STACK_CONFIG) | jq .Region) \
		--stack-name $(STACK_NAME)
	rm -f $(OUTPUT_FILE)

$(CWD)/build/proxy: $(CODE_DIR)/cmd/proxy/*.go $(SRC)
	cd $(CODE_DIR) && go build -v $(BUILD_OPT) -o $(CWD)/build/proxy $(CODE_DIR)/cmd/proxy && cd $(CWD)

proxy: $(CWD)/build/proxy
	$(CWD)/build/proxy -d $(DB_NAME) -o $(S3_OUTPUT) -r $(REGION)

$(CWD)/build/mock: $(CODE_DIR)/cmd/mock/*.go $(SRC)
	cd $(CODE_DIR) && go build -v $(BUILD_OPT) -o $(CWD)/build/mock $(CODE_DIR)/cmd/mock && cd $(CWD)

mock: $(CWD)/build/mock
	$(CWD)/build/mock