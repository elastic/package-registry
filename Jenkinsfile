#!/usr/bin/env groovy

library identifier: 'apm@current',
retriever: modernSCM(
  [$class: 'GitSCMSource',
  credentialsId: 'f94e9298-83ae-417e-ba91-85c279771570',
  id: '37cf2c00-2cc7-482e-8c62-7bbffef475e2',
  remote: 'git@github.com:elastic/apm-pipeline-library.git'])

pipeline {
  agent { label 'docker && linux && immutable' }
  environment {
    BASE_DIR="src/github.com/elastic/package-registry"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    PIPELINE_LOG_LEVEL='INFO'
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
  triggers {
    issueCommentTrigger('(?i).*(?:jenkins\\W+)?run\\W+(?:the\\W+)?tests(?:\\W+please)?.*')
  }
  stages {
    /**
    Checkout the code and stash it, to use it on other stages.
    */
    stage('Checkout') {
      steps {
        deleteDir()
        //gitCheckout(basedir: "${BASE_DIR}")
        dir("${BASE_DIR}"){
          checkout scm
        }
        stash allowEmpty: true, name: 'source', useDefaultExcludes: false
      }
    }
    /**
    Checks formatting / linting.
    */
    stage('Lint') {
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          insideGo{
            sh(label: 'Checks formatting / linting',script: 'mage -debug check  ')
          }
        }
      }
    }
    /**
    Build the project from code..
    */
    stage('Build') {
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          insideGo(){
            sh(label: 'Checks formatting / linting',script: 'mage -debug build ')
          }
        }
      }
    }
    /**
    Execute unit tests.
    */
    stage('Test') {
      steps {
        deleteDir()
        unstash 'source'
        dir("${BASE_DIR}"){
          insideGo(){
            sh(label: 'Runs the (unit) tests',script: 'mage -debug test ')
          }
        }
      }
      post {
        always {
          junit(allowEmptyResults: true,
            keepLongStdio: true,
            testResults: "${BASE_DIR}/**/junit-*.xml")
        }
      }
    }
  }
  post {
    success {
      echoColor(text: '[SUCCESS]', colorfg: 'green', colorbg: 'default')
    }
    aborted {
      echoColor(text: '[ABORTED]', colorfg: 'magenta', colorbg: 'default')
    }
    failure {
      echoColor(text: '[FAILURE]', colorfg: 'red', colorbg: 'default')
    }
    unstable {
      echoColor(text: '[UNSTABLE]', colorfg: 'yellow', colorbg: 'default')
    }
  }
}

def insideGo(Closure body){
  def goAgent = docker.build("go-agent", ".ci/jenkins-go-agent")
  goAgent.inside(){
    env.HOME="${WORKSPACE}/${BASE_DIR}"
    sh(label: 'Go version', script: 'go version')
    sh(label: 'Install Mage', script: 'mage -version')
    body()
  }
}
