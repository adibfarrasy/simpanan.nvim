MAKEFLAGS += --silent

all:
	make clean
	make build
	make prepare-copy
	make manifest
	make copy

clean:
	rm -f simpanan rplugin.vim simpanan_connections.json

build:
	go build -o simpanan .

prepare-copy:
	cat rplugin.vim.template > rplugin.vim
	echo '[]' > simpanan_connections.json

manifest:
	./simpanan -manifest simpanan >> rplugin.vim
	echo '" vim:ts=4:sw=4:et' >> rplugin.vim

copy:
	cp simpanan ~/go/bin
	cp rplugin.vim ~/.local/share/nvim

.PHONY: clean build prepare-copy manifest copy all
