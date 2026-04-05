import os
import sys

# Add the ai/ directory to sys.path so that packages like smart_router_ml
# are importable by their top-level name regardless of pytest's rootdir.
sys.path.insert(0, os.path.dirname(__file__))
