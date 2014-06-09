import argparse
from livereload import Server

parser = argparse.ArgumentParser(description='Start a documentation server')
parser.add_argument(
    '-p', '--port',
    help='Port to run docs server on',
    type=int,
    default=5500
)
args = parser.parse_args()

# Create a new application
server = Server()
server.watch('documentation/*/*rst', 'make all')
server.serve(port=args.port, root='build')
server = Server()
