# Source Attribution

Rule: k8s.mutable_image_tag

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Kubernetes images concept docs
- URL: https://kubernetes.io/docs/concepts/containers/images/
- Commit/Tag: latest
- File paths:
  - docs/concepts/containers/images
- Relevant snippet: Explains tags vs digests and deterministic image selection.

## Candidate B
- Kind: docs_example
- Title: Docker build best practices
- URL: https://docs.docker.com/build/building/best-practices/#pin-base-image-versions
- Commit/Tag: latest
- File paths:
  - build/building/best-practices/#pin-base-image-versions
- Relevant snippet: Recommends pinning versions/digests to avoid mutable tag drift.
