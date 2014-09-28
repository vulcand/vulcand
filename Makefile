.PHONY: install install-devmode run deploy all

all:
	sphinx-build -A corpsite_hostname=$(corpsite_hostname) -b html ./documentation/source ./build

install:
	pip install Sphinx
	pip install Fabric

deploy:
	git fetch origin master
	git reset --hard FETCH_HEAD
	git clean -df
	sphinx-build -A corpsite_hostname=$(corpsite_hostname) -b html ./documentation/source ./build

install-devmode:
	pip install livereload

run-devmode:
	python run.py

clean:
	rm -rf ./build
