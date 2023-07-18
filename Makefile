PROJECT_NAME := olm-bundle
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 darwin_amd64 linux_arm64 darwin_arm64
# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

-include build/makelib/output.mk

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/olm-bundle
GO_SUBDIRS += cmd internal
GO111MODULE = on
-include build/makelib/golang.mk

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

go.test.unit: $(KUBEBUILDER)

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Ensure a PR is ready for review.
reviewable: vendor lint test

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# Ensure branch is clean.
check-diff: reviewable
	@$(INFO) checking that branch is clean
	@test -z "$$(git status --porcelain)" || $(FAIL)
	@$(OK) branch is clea

.PHONY: cobertura reviewable submodules fallthrough check-diff

# Special Targets

define CROSSPLANE_TOOLS_MAKE_HELP
Crossplane Tools Targets:
    reviewable         Ensure a PR is ready for review.
    submodules         Update the submodules, such as the common build scripts.

endef
export CROSSPLANE_TOOLS_MAKE_HELP

crossplane-tools.help:
	@echo "$$CROSSPLANE_TOOLS_MAKE_HELP"

help-special: crossplane-tools.help

.PHONY: crossplane-tools.help help-special