.PHONY: install install-devmode run deploy all

all:
	sphinx-build -A corpsite_hostname=$(corpsite_hostname) -b html ./documentation/source ./build

install:
	pip install Sphinx
	pip install Fabric

deploy:
	fab -f deploy/fabfile.py deploy:$(ENV)

install-devmode:
	pip install livereload

devmode-run:
	python run.py

clean:
	rm -rf ./build
