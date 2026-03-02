include .bingo/Variables.mk

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

.PHONY: lint-hcp-rules
lint-hcp-rules: hcp-rules $(PROMTOOL) $(YQ)
	@echo ">>>>> Linting HCP tenant rules"
	./scripts/lint-hcp-rules.sh $(PROMTOOL) $(YQ)
