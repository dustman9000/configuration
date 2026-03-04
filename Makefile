include .bingo/Variables.mk

XARGS ?= $(shell which gxargs 2>/dev/null || which xargs)
OS ?= $(shell uname -s | tr '[A-Z]' '[a-z]')
OC_VERSION ?= latest
OC ?= $(GOBIN)/oc
ifeq ($(OS),darwin)
	OS = mac
endif

.PHONY: all
all: hcp-rules sc-rules lint-hcp-rules

.PHONY: hcp-rules
hcp-rules: $(YQ)
	@echo ">>>>> Generating HCP tenant rules from split files"
	YQ=$(YQ) ./scripts/generate-hcp-rules.sh

.PHONY: sc-rules
sc-rules: $(YQ)
	@echo ">>>>> Generating SC tenant rules from split files"
	YQ=$(YQ) ./scripts/generate-sc-rules.sh

.PHONY: go-lint
go-lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

.PHONY: lint-hcp-rules
lint-hcp-rules: hcp-rules $(PROMTOOL) $(YQ)
	@echo ">>>>> Linting HCP tenant rules"
	./scripts/lint-hcp-rules.sh $(PROMTOOL) $(YQ)

.PHONY: validate
validate: $(OC)
	@echo ">>>>> Validating OpenShift Templates"
	@for f in $$(find . -type f \( -name '*template.yaml' \) ! -name 'hypershift-token-refresher-template.yaml' ! -name 'hypershift-cluster-log-forwarder-template.yaml' ! -name 'hypershift-monitoring-stack-template.yaml' ! -name 'hcp_rules_template.yaml' ! -name 'ocm-component-monitoring-stack-template.yaml' ! -name 'ocm-component-token-refresher-template.yaml'); do \
		echo ">>>>> Validating $$f"; \
		$(OC) process -f "$$f" --local -o yaml > /dev/null; \
	done

$(OC): $(GOBIN)
	@echo ">>>>> Downloading OpenShift CLI (if necessary)"
	[ -x $(OC) ] || curl -sNL "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/$(OC_VERSION)/openshift-client-$(OS).tar.gz" | tar -xzf - -C $(GOBIN)
