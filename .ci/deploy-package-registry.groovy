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
    DOCKER_IMG_SOURCE = "${env.DOCKER_REGISTRY}/package-registry/distribution:production"
    DOCKER_IMG_TARGET = "${env.DOCKER_REGISTRY}/package-registry/distribution:${env.DOCKER_TAG}"
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
        // Validate only semver are allowd for the docker tag.
        // It's allowed to override existing published docker images. For instance, the build candidates generated
        // by the unified release process will share the same versioning, therefore the Git release tag will be
        // the last docker image to be republished.
        whenFalse(isSemVerValid(env.DOCKER_TAG)) {
          error('unsupported docker tag, please use the major.minor.path(-prerelease)? format (for example: 1.2.3 or 1.2.3-alpha).')
        }
      }
    }
    stage('Publish Docker image'){
      options { skipDefaultCheckout() }
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
