.PHONY: install install-local update update-local

install:
	go build -o shef
	@echo "Installing Shef to /usr/local/bin (may require sudo password)"
	@sudo mv shef /usr/local/bin/ || (echo "Failed to install. Try: sudo make install" && exit 1)
	@echo "Installation complete. Run 'shef -h' to verify."

install-local:
	go build -o shef
	@mkdir -p $(HOME)/bin
	@mv shef $(HOME)/bin/
	@echo "Installed to $(HOME)/bin/shef"
	@echo "Make sure $(HOME)/bin is in your PATH"
	@echo "Example: export PATH=\"\$$PATH:\$$HOME/bin\""

update:
	go build -o shef
	@echo "Updating Shef in /usr/local/bin (may require sudo password)"
	@sudo mv shef /usr/local/bin/ || (echo "Failed to update. Try: sudo make update" && exit 1)
	@echo "Update complete. Run 'shef -h' to verify."

update-local:
	go build -o shef
	@mkdir -p $(HOME)/bin
	@mv shef $(HOME)/bin/
	@echo "Updated to $(HOME)/bin/shef"
	@echo "Make sure $(HOME)/bin is in your PATH"
	@echo "Example: export PATH=\"\$PATH:\$HOME/bin\""
