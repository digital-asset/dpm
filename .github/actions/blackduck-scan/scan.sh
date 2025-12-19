#!/usr/bin/env bash

set -eux

# these are all environment variables set in GHA specific to the git repo
# they are not available when running locally so add some defaults, and grab branch dynamically

export GITHUB_REPOSITORY=${GITHUB_REPOSITORY:-"DACH-NY"}
export GITHUB_JOB=${GITHUB_JOB:-"local"}
export GITHUB_REF_NAME=${GITHUB_REF_NAME:-$(git branch --show-current)}


#default platform all if not specified
platform=go
extra_flags=""

Help()
 {
    # Display Help
    echo "Run Blackduck scan tailored to particular build tool ecosystem"
    echo
    echo "-p|--platform
      This is the build tool ecosystem platform you would like to scan
      Value should be passed in with a space

      Examples :
      ./blackduck-scan.sh -p go
      ./blackduck-scan.sh --platform go
      "
    echo
    echo "-d|--diagnostic
      Enable diagnostic logging to provide comprehensive detail to synopsys support
      "
 }

 EnableDiagnostic() {
   extra_flags+=" --detect.diagnostic=true "
 }

#support parameters
while [[ $# -gt 0 ]]; do
  case $1 in
    -p|--platform)
      platform="$2"
      shift # past argument
      shift # past value
      ;;
    -d|--diagnostic)
      EnableDiagnostic
      shift
      ;;
    -h|--help) # display Help
      Help
      exit;;
    -*)
      echo "Unknown option $1"
      exit 1
      ;;
  esac
done

# ignore exit code for grep that checks for changed lockfiles
set +e
NIX_LOCK_FILES_PATTERN='|shell.nix|flake.lock'

case "${platform}" in
  "python")
    LOCK_FILE_PATTERN="poetry.lock${NIX_LOCK_FILES_PATTERN}"
    extra_flags+="
    --detect.included.detector.types=PIP,POETRY"
    ;;
  "go")
    LOCK_FILE_PATTERN="go.sum|go.mod${NIX_LOCK_FILES_PATTERN}"
    extra_flags+="
    --detect.included.detector.types=GO_MOD \
    --detect.go.mod.dependency.types.excluded=UNUSED \
    --detect.excluded.directories=\"/^(?!go).*/m\""
    ;;
  "jvm")
    LOCK_FILE_PATTERN="build.sbt.lock${NIX_LOCK_FILES_PATTERN}"
    extra_flags+="
    --detect.included.detector.types=SBT,MAVEN \
    --detect.sbt.arguments=-Dsbt.log.noformat=true \
    --detect.required.detector.types=SBT"
    ;;
  "npm")
    LOCK_FILE_PATTERN="package.json|package-lock.json${NIX_LOCK_FILES_PATTERN}"
    extra_flags+="
    --detect.excluded.directories=cli/lib \
    --detect.included.detector.types=NPM \
    --detect.npm.dependency.types.excluded=DEV \
    --detect.npm.arguments=--silent \
    --detect.yarn.dependency.types.excluded=NON_PRODUCTION \
    --detect.excluded.directories=tools/installer/bats/"
    ;;
  "docker")
    LOCK_FILE_PATTERN=""  #match everything
    extra_flags+="
      --detect.tools=DETECTOR,DOCKER,SIGNATURE_SCAN \
      --detect.docker.image=${IMAGE_TAG} \
      --detect.docker.passthrough.cleanup.inspector.container=true \
      --detect.detector.search.continue=true \
      --detect.detector.search.depth=5 \
      --detect.tools.excluded=BINARY_SCAN \
      --detect.target.type=IMAGE"
    ;;
  "all")
    LOCK_FILE_PATTERN="poetry.toml|poetry.lock|go.sum|go.mod|build.sbt.lock|package.json|package-lock.json|pom.xml${NIX_LOCK_FILES_PATTERN}"  #match everything
    extra_flags+="
    --detect.included.detect.types=POETRY,GO_MOD,NPM,SBT,MAVEN \
    --detect.go.mod.dependency.types.excluded=UNUSED \
    --detect.excluded.directories=\"/^(?!go).*/m\" \
    --detect.sbt.arguments=-Dsbt.log.noformat=true \
    --detect.excluded.directories=cli/lib \
    --detect.npm.dependency.types.excluded=DEV \
    --detect.npm.arguments=--silent \
    --detect.yarn.dependency.types.excluded=NON_PRODUCTION \
    --detect.excluded.directories=tools/installer/bats/"
    ;;
*)
  echo "platform $platform is unknown to us"
  exit 1
;;
esac
set -eux

# RAPID scan provides quicker feedback so run this on PRs -- run full intelligent persisted scan upon main merge
BRANCH_FOR_FULL_SCAN="main"
if [ "${GITHUB_REF_NAME}" == "${BRANCH_FOR_FULL_SCAN}" ]; then
  # grep returns exit code 1 when no match, so ignore that, we just want the count of lockfile matches
  LOCK_FILE_DETECTED="$(git --no-pager diff --name-only origin/main HEAD~1|grep -Ec $LOCK_FILE_PATTERN || :)"
  SCAN_MODE="INTELLIGENT"
  COMPARE_MODE="ALL"
  extra_flags+=" || true"
else
  # grep returns exit code 1 when no match, so ignore that, we just want the count of lockfile matches
  LOCK_FILE_DETECTED="$(git --no-pager diff --name-only origin/main|grep -Ec $LOCK_FILE_PATTERN || :)"
  SCAN_MODE="RAPID"
  COMPARE_MODE="BOM_COMPARE"
fi

# Only run when on main or when a lock file specific to that ecosystem is changed and the job name matches one of the apps in jobs_to_run_scan
if [[ "${LOCK_FILE_DETECTED}" -gt 0 ]]; then
  bash <(curl -s https://raw.githubusercontent.com/DACH-NY/security-blackduck/master/synopsys-detect) \
    ci-build "${GITHUB_REPOSITORY}" "${GITHUB_REF_NAME}" \
    --logging.level.com.synopsys.integration=DEBUG \
    --detect.tools=DETECTOR \
    --detect.notices.report=false \
    --detect.code.location.name="${GITHUB_REPOSITORY}"_"${GITHUB_REF_NAME}"_"${GITHUB_JOB}" \
    --detect.blackduck.scan.mode="${SCAN_MODE}" \
    --detect.blackduck.rapid.compare.mode="${COMPARE_MODE}" \
    --detect.cleanup=false \
    --detect.timeout=1200 \
    "$extra_flags"
else
  echo "scan is only run when a lock file change is detected"
  echo "current branch is $GITHUB_REF_NAME and job is $GITHUB_JOB and lock file detected is $LOCK_FILE_DETECTED"
fi
