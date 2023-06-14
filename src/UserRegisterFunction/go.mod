require github.com/aws/aws-lambda-go v1.35.0

// GitHub blocks the gopkg.in proxy service. Replace dependency modules with
// references to the source repos.
replace (
  gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20201130134442-10cb98267c6c
  gopkg.in/yaml.v3 => github.com/go-yaml/yaml/v3 v3.0.1
)

module Function