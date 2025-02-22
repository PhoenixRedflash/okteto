#!/usr/bin/env bash

# release.sh is the release script to make Okteto CLI versions publicly
# available.
#
# This scripts is meant to be executed by circleci and makes a few assumptions about
# the environment it runs on. It assumes the golangci executor which has a few
# binaries required by this script.
# TODO: parameterize this script to make it able to run locally
#
# Releases can be pulled from three distinct channels: stable, beta and dev.
#
# Releases are represented by git annotated tags in semver version and have their
# respective Github release.
# Releases from the stable channel have the format: MAJOR.MINOR.PATCH
# Releases from the beta channel have the MAJOR.MINOR.PATCH-beta.n
# Releases from the dev channel have the MAJOR.MINOR.PATCH-dev.n
#

# run in a subshell
{ (

        set -e # make any error fail the script
        set -u # make unbound variables fail the script

        # SC2039: In POSIX sh, set option pipefail is undefined
        # shellcheck disable=SC2039
        set -o pipefail # make any pipe error fail the script

        # RELEASE_TAG is the release tag that we want to release
        RELEASE_TAG="${CIRCLE_TAG:-""}"

        # RELEASE_COMMIT is the commit being released
        RELEASE_COMMIT=${CIRCLE_SHA1}

        # PSEUDO_TAG is the short sha that will be used to release in the case
        # that a release tag is not provided
        PSEUDO_TAG="$(echo "$RELEASE_COMMIT" | head -c 7)"

        # CURRENT_BRANCH is the branch being released.
        CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

        # REPO_OWNER is the owner of the repo (okteto)
        REPO_OWNER="${CIRCLE_PROJECT_USERNAME}"

        # REPO_NAME is the name of the repo (okteto)
        REPO_NAME="${CIRCLE_PROJECT_REPONAME}"

        # BIN_PATH is where the artifacts are stored. Usually precreated by the circleci
        # workflow.
        BIN_PATH="$PWD/artifacts/bin"

        ################################################################################
        # Sanity check
        ################################################################################

        if ! semver --version >/dev/null 2>&1; then
                echo "Binary \"semver\" does is required from running this scripts"
                echo "Please install it and try again:"
                echo "  $ curl -o /usr/local/bin/semver https://raw.githubusercontent.com/fsaintjacques/semver-tool/master/src/semver"
                exit 1
        fi

        if ! command -v okteto-ci-utils >/dev/null 2>&1; then
                echo "Binary \"okteto-ci-utils\" from the golangci circleci executor is required to run this script"
                echo "If you are running this locally you can go into the golang-ci executor repo and build the script from source:"
                echo "  $ go build -o /usr/local/bin/okteto-ci-utils ."
                exit 1
        fi

        if ! command -v gsutil >/dev/null 2>&1; then
                echo "Binary \"gsutils\" from Google Cloud is required to run this script. Find installation instructions at https://cloud.google.com/sdk/docs/install"
                exit 1
        fi

        if ! command -v ghr >/dev/null 2>&1; then
                echo "Binary \"ghr\" is required to run this script. Install with:"
                echo "  $ GOPROXY=direct GOSUMDB=off go install github.com/tcnksm/ghr@latest"
                exit 1
        fi

        if [ ! -d "$BIN_PATH" ]; then
                echo "Release artifacts should be stored in $BIN_PATH but the directory does no exist"
                exit 1
        elif [ -z "$(ls -A "$BIN_PATH")" ]; then
                echo "Release artifacts should be stored in $BIN_PATH but the directory is empty"
                exit 1
        fi

        if [ -z "$GITHUB_TOKEN" ]; then
                echo "GITHUB_TOKEN envvar not provided. It is required to create the Github Release and the release notes"
                exit 1
        fi

        echo "Releasing tag '${RELEASE_TAG}' from branch '${CURRENT_BRANCH}' at ${RELEASE_COMMIT}"

        ################################################################################
        # Resolve release channel
        ################################################################################

        # Resolve the channel from the tag that is being released
        # If the channel is unknown the release will fail
        CHANNELS=

        # dev releases don't have tags
        if [ "$RELEASE_TAG" = "" ]; then
                CHANNELS=("dev")
        else
                beta_prerel_regex="^beta\.[0-9]+"
                prerel="$(semver get prerel "${RELEASE_TAG}")"

                # Stable releases are added to all channel
                if [ -z "$prerel" ]; then
                        CHANNELS=("stable" "beta" "dev")

                elif [[ $prerel =~ $beta_prerel_regex ]]; then
                        CHANNELS=("beta" "dev")

                else
                        echo "Unknown tag"
                        echo "Expected one of: "
                        echo "  - stable: MAJOR.MINOR.PATCH "
                        echo "  - beta: MAJOR.MINOR.PATCH-beta.n"
                        echo "$RELEASE_TAG matches none"
                        exit 1
                fi
        fi

        for chan in "${CHANNELS[@]}"; do
                echo "---------------------------------------------------------------------------------"
                tag="${RELEASE_TAG:-"$PSEUDO_TAG"}"
                echo "Releasing ${tag} into ${chan} channel"

                ##############################################################################
                # Update downloads.okteto.com
                ##############################################################################

                # BIN_BUCKET_NAME is the name of the bucket where the binaries are stored.
                # Starting at Okteto CLI 2.0, all these binaries are publicly accessible at:
                # https://downloads.okteto.com/cli/<channel>/<tag>
                BIN_BUCKET_ROOT="downloads.okteto.com/cli/${chan}"
                BIN_BUCKET_NAME="${BIN_BUCKET_ROOT}/${tag}"

                # VERSIONS_BUCKET_NAME are all the available versions for a release channel.
                # This is also publicly accessible at:
                # https://downloads.okteto.com/cli/<channel>/versions
                VERSIONS_BUCKET_NAME="downloads.okteto.com/cli/${chan}/versions"

                # upload artifacts
                echo "Syncing artifacts from $BIN_PATH with $BIN_BUCKET_NAME"
                gsutil -m rsync -r "$BIN_PATH" "gs://$BIN_BUCKET_NAME"

                # Get the current versions file and add the current version being released.
                # These are the versions publicly accessible from this channel.
                # It is important to have them sorted so that the last version from the list
                # is always the latest and we can keep pushing older tags for maintenance and
                # whatnot.
                version_temp_file=$(mktemp)
                gsutil cat "gs://${VERSIONS_BUCKET_NAME}" >"$version_temp_file"
                echo "Current version list for ${chan} channel (showing latest 10):"
                tail "$version_temp_file" -n 10

                printf "%s\n" "${tag}" >>"$version_temp_file"

                # dont sort the dev channel. Not all tags are semver and it's
                # safe to assume linear history
                if [ "${chan}" = "dev" ]; then
                        # SC2002: Useless cat. Consider 'cmd < file | ..' or 'cmd file | ..' instead
                        # shellcheck disable=SC2002
                        cat "$version_temp_file" >"${BIN_PATH}/versions"
                else
                        # remove duplicated versions and sort the list
                        # SC2002: Useless cat. Consider 'cmd < file | ..' or 'cmd file | ..' instead
                        # shellcheck disable=SC2002
                        cat "$version_temp_file" | awk '!seen[$0]++' | okteto-ci-utils semver-sort >"${BIN_PATH}/versions"
                fi

                echo "Added ${tag} to the version list"
                echo "New version list for ${chan} channel (showing latest 10):"
                tail "${BIN_PATH}/versions" -n 10

                # After sorting, if the latest tag is the current tag update the root path
                # with the current binaries
                latest="$(tail "${BIN_PATH}/versions" -n1)"

                if [ "$tag" = "$latest" ]; then
                        gsutil -m rsync "gs://$BIN_BUCKET_NAME" "gs://$BIN_BUCKET_ROOT"
                fi

                gsutil -m -h "Cache-Control: no-store" -h "Content-Type: text/plain" cp "${BIN_PATH}/versions" "gs://${VERSIONS_BUCKET_NAME}"
                echo "${chan} channel updated with ${tag}"
        done

        if [ "$RELEASE_TAG" = "" ]; then
                echo "No RELEASE_TAG, skipping github release for pseudo tag ${PSEUDO_TAG} from ${CURRENT_BRANCH}"
                echo "All done"
                exit 0
        fi

        ################################################################################
        # Update Github Release
        ################################################################################
        previous_version=$(grep -F "$RELEASE_TAG" -B 1 "${BIN_PATH}/versions" | head -n1)

        # SC2116: Useless echo? Instead of 'cmd $(echo foo)', just use 'cmd foo'.
        # SC2128: Expanding an array without an index only gives the first element.
        # shellcheck disable=SC2116,SC2128
        preferred_channel="$(echo "$CHANNELS")"

        echo "Gathering ${RELEASE_TAG} release notes. Diffing from ${previous_version}"
        notes=$(curl \
                -fsS \
                -X POST \
                -H "Authorization: Bearer ${GITHUB_TOKEN}" \
                -H "Accept: application/vnd.github.v3+json" \
                -d "{\"tag_name\":\"$RELEASE_TAG\",\"previous_tag_name\":\"$previous_version\"}" \
                "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/generate-notes" | jq .body)

        printf "RELEASE NOTES:\n%s" "${notes}"

        prerelease=false
        name="Okteto CLI - ${RELEASE_TAG}"
        if [ "${preferred_channel}" = "beta" ]; then
                prerelease=true
                name="Okteto CLI [${preferred_channel}] - ${RELEASE_TAG}"
        fi

        echo "Using ghr version: $(ghr -version)"
        ghr \
                -u "${REPO_OWNER}" \
                -n "${name}" \
                -r "${REPO_NAME}" \
                -c "${RELEASE_COMMIT}" \
                -token "${GITHUB_TOKEN}" \
                -b "${notes}" \
                -replace \
                -prerelease="${prerelease}" \
                "${RELEASE_TAG}" \
                "${BIN_PATH}"

        echo "Created Github release: '${name}'"

        echo "All done"
); }
