# Makefile to set up go.mod for all examples

.PHONY: setup

setup:
	@echo "Setting up go.mod files for all examples..."
	@find . -name "main.go" -type f | while read f; do \
		dir=$$(dirname "$$f"); \
		if [ ! -f "$$dir/go.mod" ]; then \
			relpath=$$(echo "$$dir" | sed 's|[^/]*|..|g'); \
			cd "$$dir" && \
			go mod init $$(basename "$$dir") 2>/dev/null && \
			echo "replace clix => $$relpath/../.." >> go.mod && \
			echo "✓ Created go.mod for $$dir"; \
		fi; \
	done
	@echo "Done!"

