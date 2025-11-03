# Makefile to set up go.mod for all examples

.PHONY: setup

setup:
	@echo "Setting up go.mod files for all examples..."
	@find . -name "main.go" -type f | while read f; do \
		dir=$$(dirname "$$f"); \
		if [ ! -f "$$dir/go.mod" ]; then \
			cd "$$dir" && \
			go mod init $$(basename "$$dir") 2>/dev/null || true && \
			# Calculate depth: commands/example1 = 1, code/commands/example1 = 2, docs/code/commands/example1 = 3
			# Need 4 levels: example1 -> commands -> code -> docs -> root
			path="../../../../"; \
			echo "replace clix => $$path" >> go.mod && \
			echo "require clix v0.0.0" >> go.mod && \
			cd - > /dev/null && \
			echo "✓ Created go.mod for $$dir"; \
		fi; \
	done
	@echo "Done!"

