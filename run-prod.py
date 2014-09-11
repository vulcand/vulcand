from flask import Flask, request, redirect
import os
from urlparse import urlparse, urlunparse

# set the project root directory as the static folder, you can set others.
app = Flask(__name__, static_url_path='', static_folder='build')

@app.before_request
def redirect_nonhttps():
    return
    """Redirect non-www requests to https"""
    if request.headers['X-Forwarded-Proto'] == 'https':
        return
    u = list(urlparse(request.url))
    u[0] = 'https'
    return redirect(urlunparse(u), code=301)

@app.route('/')
def root():
    return app.send_static_file('index.html')


@app.route('/<path:path>')
def static_proxy(path):
    # send_static_file will guess the correct MIME type
    return app.send_static_file(path)


if __name__ == '__main__':
    app.run(port=5501)
