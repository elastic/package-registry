#!/usr/bin/env groovy

@Library('apm@current') _

pipeline {
  agent { label 'ubuntu-20' }
  environment {
    BASE_DIR = "src/github.com/elastic/package-registry"
    PIPELINE_LOG_LEVEL = 'INFO'
    DOCKER_REGISTRY = 'docker.elastic.co'
    DOCKER_REGISTRY_SECRET = 'secret/observability-team/ci/docker-registry/prod'
    DOCKER_TAG = "${params.DOCKER_TAG}"
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
    quietPeriod(10)
  }
  parameters {
    string(name: 'DOCKER_TAG', defaultValue: 'latest', description: 'The docker tag to be published (format: major.minor.patch(-prerelease)?).')
  }
  stages {
    stage('Validate docker tag'){
      options { skipDefaultCheckout() }
      steps {
        transformTagAndValidate()
      }
    }
    stage('Publish Production Docker image'){
      options { skipDefaultCheckout() }
      environment {
        DOCKER_IMG_SOURCE = "${env.DOCKER_REGISTRY}/package-registry/distribution:production"
        DOCKER_IMG_TARGET = "${env.DOCKER_REGISTRY}/package-registry/distribution:${env.DOCKER_TAG_VERSION}"
      }
      steps {
        dockerLogin(secret: "${env.DOCKER_REGISTRY_SECRET}", registry: "${env.DOCKER_REGISTRY}")
        retryWithSleep(retries: 3, seconds: 5, backoff: true) {
          sh(label: 'Docker pull', script: 'docker pull ${DOCKER_IMG_SOURCE}')
        }
        sh(label: 'Docker retag',  script: 'docker tag ${DOCKER_IMG_SOURCE} ${DOCKER_IMG_TARGET}')
        retryWithSleep(retries: 3, seconds: 5, backoff: true) {
          sh(label: 'Docker push', script: 'docker push ${DOCKER_IMG_TARGET}')
        }
      }
    }
    stage('Publish Lite Docker image'){
      options { skipDefaultCheckout() }
      environment {
        DOCKER_IMG_SOURCE = "${env.DOCKER_REGISTRY}/package-registry/distribution:lite"
        DOCKER_IMG_TARGET = "${env.DOCKER_REGISTRY}/package-registry/distribution:lite-${env.DOCKER_TAG_VERSION}"
      }
      steps {
        dockerLogin(secret: "${env.DOCKER_REGISTRY_SECRET}", registry: "${env.DOCKER_REGISTRY}")
        retryWithSleep(retries: 3, seconds: 5, backoff: true) {
          sh(label: 'Docker pull', script: 'docker pull ${DOCKER_IMG_SOURCE}')
        }
        sh(label: 'Docker retag',  script: 'docker tag ${DOCKER_IMG_SOURCE} ${DOCKER_IMG_TARGET}')
        retryWithSleep(retries: 3, seconds: 5, backoff: true) {
          sh(label: 'Docker push', script: 'docker push ${DOCKER_IMG_TARGET}')
        }
      }
    }
  }
  post {
    cleanup {
      notifyBuildResult(prComment: false)
    }
  }
}

def isSemVerValid(String version) {
  def match = version =~ /\d+.\d+.\d+(-\w+)?/
  return match.matches()
}

/**
* Transform the DOCKER_TAG to be a valid docker tag format in case
* it contains the git tag with the 'v' prefix
*/
def transformTagAndValidate() {
  // If the docker tag contains the 'v' prefix, then remove it.
  // fleet-server and other projects use tag releases with v<major>.<minor>.<patch>
  // i.e: v8.3.1
  def version = env.DOCKER_TAG.replaceAll('^v', '')
  // Validate only semver are allowed for the docker tag.
  // It's allowed to override existing published docker images. For instance, the build candidates generated
  // by the unified release process will share the same versioning, therefore the Git release tag will be
  // the last docker image to be republished.
  if (!isSemVerValid(version)) {
    error('unsupported docker tag, please use the major.minor.path(-prerelease)? format (for example: 1.2.3 or 1.2.3-alpha).')
  }
  env.DOCKER_TAG_VERSION = version
}
