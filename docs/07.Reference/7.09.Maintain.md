# Release steps

These are the steps to release a new version of Easegress.

## Pre-requisites

Let's suppose that the next version is `X.Y.Z`. Then new *tag* is `vX.Y.Z` (note prefix `v`) and new *release title* is `easegress-vX.Y.Z`. Please collect all Pull Request's that will be mentioned in `CHANGELOG.md` and in the Release Note.

## Steps

1. Create a Pull Request that updates release version `X.Y.Z` in Makefile and adds the new version to `CHANGELOG.md`. Include significant changes, implemented enchantments and fixed bugs to `CHANGELOG.md`. Once the PR is reviewed and approved, merge it to main branch.
2. Create new release note at https://github.com/megaease/easegress/releases/new with *release title* `easegress-vX.Y.Z`, click *Choose a tag* and add new tag `vX.Y.Z`. Write the Release note. You can use the modifications written to `CHANGELOG.md` in previous step. Follow the same format, as previous Release notes. Once written, click `Save draft`. 
3. New tag triggers Goreleaser Github action. Follow up that it creates and uploads successfully all binaries and Docker images. 
4. Ensure that the release note created in step 2 contains the command to pull the new Docker image.
5. Edit the release note create in step 2, and click `Publish release`. 
6. New version of Easegress is now released! Notify everyone to check it out!
