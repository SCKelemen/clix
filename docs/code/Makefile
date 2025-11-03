# Makefile to set up go.mod for all examples

.PHONY: setup

setup:
	@echo "Setting up go.mod files for all examples..."
	@find . -name "main.go" -type f | while read f; do \
		dir=$$(dirname "$$f"); \
		if [ ! -f "$$dir/go.mod" ]; then \
			cd "$$dir" && \
			go mod init $$(basename "$$dir") 2>/dev/null || true && \
			depth=$$(echo "$$dir" | tr -cd '/' | wc -c | tr -d ' '); \
			depth=$$((depth + 1)); \
			path=$$(printf '../%.0s' $$(seq 1 $$depth)); \
			echo "replace clix => $$path" >> go.mod && \
			echo "require clix v0.0.0" >> go.mod && \
			cd - > /dev/null && \
			echo "✓ Created go.mod for $$dir"; \
		fi; \
	done
	@echo "Done!"

