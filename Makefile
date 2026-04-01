include .bingo/Variables.mk

XARGS ?= $(shell which gxargs 2>/dev/null || which xargs)
OS ?= $(shell uname -s | tr '[A-Z]' '[a-z]')
OC_VERSION ?= latest
OC ?= $(GOBIN)/oc
ifeq ($(OS),darwin)
	OS = mac
endif

.PHONY: all
all: hcp-rules sc-rules hcp-loki-rules

.PHONY: hcp-rules
hcp-rules: generate-hcp-rules lint-hcp-rules

.PHONY: generate-hcp-rules
generate-hcp-rules: $(YQ)
	@echo ">>>>> Generating HCP tenant rules from split files"
	YQ=$(YQ) ./scripts/generate-hcp-rules.sh

.PHONY: lint-hcp-rules
lint-hcp-rules: $(PROMTOOL) $(YQ)
	@echo ">>>>> Linting HCP tenant rules"
	./scripts/lint-rules.sh ./resources/tenant-rules/hcp.yaml $(PROMTOOL) $(YQ)

.PHONY: sc-rules
sc-rules: generate-sc-rules lint-sc-rules

.PHONY: generate-sc-rules
generate-sc-rules: $(YQ)
	@echo ">>>>> Generating SC tenant rules from split files"
	YQ=$(YQ) ./scripts/generate-sc-rules.sh

.PHONY: lint-sc-rules
lint-sc-rules: $(PROMTOOL) $(YQ)
	@echo ">>>>> Linting SC tenant rules"
	./scripts/lint-rules.sh ./resources/tenant-rules/sc.yaml $(PROMTOOL) $(YQ)

.PHONY: hcp-loki-rules
hcp-loki-rules: generate-hcp-loki-rules lint-hcp-loki-alerting-rules

.PHONY: generate-hcp-loki-rules
generate-hcp-loki-rules: $(YQ)
	@echo ">>>>> Generating HCP Loki AlertingRule template"
	YQ=$(YQ) ./scripts/generate-hcp-loki-rules.sh

.PHONY: lint-hcp-loki-alerting-rules
lint-hcp-loki-alerting-rules: $(LOGCLI) $(YQ)
	@echo ">>>>> Linting HCP Loki AlertingRule template"
	./scripts/lint-loki-rules.sh ./resources/tenant-rules/hcp-loki-alerting-rules.yaml $(LOGCLI) $(YQ)

.PHONY: yaml-lint
yaml-lint: $(YQ)
	@echo ">>>>> Validating YAML syntax in resources/"
	@ERRORS=0; \
	for f in $$(find resources/ -name '*.yaml' -o -name '*.yml'); do \
		if ! $(YQ) eval '.' "$$f" > /dev/null 2>&1; then \
			echo "INVALID: $$f"; \
			$(YQ) eval '.' "$$f" 2>&1 | head -5; \
			ERRORS=$$((ERRORS + 1)); \
		fi; \
	done; \
	if [ $$ERRORS -gt 0 ]; then echo "$$ERRORS file(s) failed YAML validation"; exit 1; fi; \
	echo "All YAML files valid"

.PHONY: go-lint
go-lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix

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
