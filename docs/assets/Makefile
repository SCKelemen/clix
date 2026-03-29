# Makefile for recording and converting terminal animations

.PHONY: help install-tools record convert clean

help:
	@echo "Terminal Animation Recording Tools"
	@echo ""
	@echo "Commands:"
	@echo "  make install-tools    - Install required tools (asciinema, agg, ffmpeg)"
	@echo "  make record NAME=xxx  - Record a new animation (interactive)"
	@echo "  make convert FILE=xxx - Convert existing GIF to WebP"
	@echo "  make clean            - Remove intermediate files (.cast, .gif)"

install-tools:
	@echo "Installing tools..."
	@if command -v brew > /dev/null; then \
		brew install asciinema agg ffmpeg; \
	elif command -v apt-get > /dev/null; then \
		sudo apt-get update && sudo apt-get install -y asciinema agg ffmpeg; \
	elif command -v npm > /dev/null; then \
		npm install -g asciinema-cli agg; \
		brew install ffmpeg || sudo apt-get install -y ffmpeg; \
	else \
		echo "Please install manually: asciinema, agg, and ffmpeg"; \
		exit 1; \
	fi
	@echo "✓ Tools installed"

record:
	@if [ -z "$(NAME)" ]; then \
		echo "Usage: make record NAME=commands_0"; \
		exit 1; \
	fi
	@./record.sh $(NAME)

convert:
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make convert FILE=animation.gif"; \
		exit 1; \
	fi
	@./convert_to_webp.sh $(FILE)

clean:
	@echo "Cleaning intermediate files..."
	@rm -f *.cast *.gif
	@echo "✓ Cleaned"

