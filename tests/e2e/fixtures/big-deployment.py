#!/usr/bin/env python3
"""Generate a large Deployment manifest for payload-size testing."""
try:
    import yaml
except ImportError:
    yaml = None
import json

containers = []
for i in range(5):
    containers.append({
        'name': f'worker-{i}',
        'image': f'myapp/worker:v1.{i}.0',
        'resources': {
            'limits': {'cpu': '500m', 'memory': '256Mi'},
            'requests': {'cpu': '100m', 'memory': '128Mi'},
        },
        'env': [
            {'name': 'WORKER_ID', 'value': str(i)},
            {'name': 'LOG_LEVEL', 'value': 'info'},
        ],
        'ports': [{'containerPort': 8080 + i}],
        'securityContext': {'runAsNonRoot': True, 'readOnlyRootFilesystem': True},
    })

manifest = {
    'apiVersion': 'apps/v1',
    'kind': 'Deployment',
    'metadata': {
        'name': 'big-deployment',
        'namespace': 'staging',
        'labels': {'app': 'big-app', 'version': 'v1.0'},
    },
    'spec': {
        'replicas': 3,
        'selector': {'matchLabels': {'app': 'big-app'}},
        'template': {
            'metadata': {'labels': {'app': 'big-app'}},
            'spec': {'containers': containers},
        },
    },
}
if yaml:
    print(yaml.dump(manifest, default_flow_style=False))
else:
    # Fallback: emit YAML-compatible JSON (valid YAML superset)
    print(json.dumps(manifest, indent=2))
