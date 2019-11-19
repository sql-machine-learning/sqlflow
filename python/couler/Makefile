SQLFLOW_VERSION := develop
VERSION_FILE := couler/_version.py
SHELL := /bin/bash

setup: ## Setup virtual environment for local development
	python3 -m venv venv
	source venv/bin/activate \
	&& $(MAKE) install-requirement

install-requirements:
	pip install -U -e .

test: ## Run tests
	python3 setup.py test

clean: ## Clean up temporary folders
	rm -rf build dist .eggs *.egg-info .pytest_cache

release: ## Release new version
	$(if $(shell git status -s), $(error "Please commit your changes or stash them before you release."))

	# Make sure local develop branch is up-to-date
	git fetch origin
	git checkout develop
	git merge origin/develop

	# Remove dev from version number
	$(eval VERSION := $(subst .dev,,$(shell python -c "exec(open('$(VERSION_FILE)').read());print(__version__)")))
	$(info release $(VERSION)...)
	sed -i '' "s/, 'dev'//" $(VERSION_FILE)
	git commit -a -m "release $(VERSION)"

	# Tag it
	git tag v$(VERSION)

	# Bump version for development
	$(eval NEXT_VERSION := $(shell echo $(VERSION) | awk -F. '{print $$1"."($$2+1)".0"}'))
	$(eval VERSION_CODE := $(shell echo $(NEXT_VERSION) | sed 's/\./, /g'))
	sed -i '' -E "s/[0-9]+, [0-9]+, [0-9]+/$(VERSION_CODE), 'dev'/" $(VERSION_FILE)
	git commit -a -m "start $(NEXT_VERSION)"
	git push origin develop

	# Push the tag, release the package to pypi
	git push --tags

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: help doc
.DEFAULT_GOAL := help
