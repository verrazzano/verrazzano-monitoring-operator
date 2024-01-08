// Copyright (c) 2020, 2024, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

def DOCKER_IMAGE_TAG

pipeline {
    options {
        skipDefaultCheckout true
    }

    agent {
       docker {
            image "${RUNNER_DOCKER_IMAGE}"
            args "${RUNNER_DOCKER_ARGS}"
            registryUrl "${RUNNER_DOCKER_REGISTRY_URL}"
            registryCredentialsId 'ocir-pull-and-push-account'
            label "2.0-large"
       }
    }

    environment {
        DOCKER_CI_IMAGE_NAME_OPERATOR = 'verrazzano-monitoring-operator-jenkins'
        DOCKER_PUBLISH_IMAGE_NAME_OPERATOR = 'verrazzano-monitoring-operator'
        DOCKER_IMAGE_NAME_OPERATOR = "${env.BRANCH_NAME ==~ /^release-.*/ || env.BRANCH_NAME == 'master' ? env.DOCKER_PUBLISH_IMAGE_NAME_OPERATOR : env.DOCKER_CI_IMAGE_NAME_OPERATOR}"

        DOCKER_CI_IMAGE_NAME_ESWAIT = 'verrazzano-monitoring-instance-eswait-jenkins'
        DOCKER_PUBLISH_IMAGE_NAME_ESWAIT = 'verrazzano-monitoring-instance-eswait'
        DOCKER_IMAGE_NAME_ESWAIT = "${env.BRANCH_NAME == 'master' ? env.DOCKER_PUBLISH_IMAGE_NAME_ESWAIT : env.DOCKER_CI_IMAGE_NAME_ESWAIT}"

        CREATE_LATEST_TAG = "${env.BRANCH_NAME == 'master' ? '1' : '0'}"
        GOPATH = '/home/opc/go'
        GO_REPO_PATH = "${GOPATH}/src/github.com/verrazzano"
        DOCKER_CREDS = credentials('github-packages-credentials-rw')
        DOCKER_REPO = 'ghcr.io'
        DOCKER_NAMESPACE = 'verrazzano'
        HELM_CHART_NAME = 'verrazzano-monitoring-operator'
        VMI_NAMESAPCE_PREFIX = 'vmi'
        INGRESS_NODE_PORT = sh(script: "shuf -i 30000-32767 -n 1" , returnStdout: true)
        KUBECONFIG = '~/.kube/config'
        NETRC_FILE = credentials('netrc')
    }

    stages {
        stage('Clean workspace and checkout') {
            steps {
                checkout scm
                sh """
                    cp -f "${NETRC_FILE}" $HOME/.netrc
                    chmod 600 $HOME/.netrc
                """
      	        sh """
                    echo "${DOCKER_CREDS_PSW}" | docker login ${env.DOCKER_REPO} -u ${DOCKER_CREDS_USR} --password-stdin
                    rm -rf ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    mkdir -p ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    tar cf - . | (cd ${GO_REPO_PATH}/verrazzano-monitoring-operator/ ; tar xf -)
                """
                script {
                    def props = readProperties file: '.verrazzano-development-version'
                    VERRAZZANO_DEV_VERSION = props['verrazzano-development-version']
                    TIMESTAMP = sh(returnStdout: true, script: "date +%Y%m%d%H%M%S").trim()
                    SHORT_COMMIT_HASH = sh(returnStdout: true, script: "git rev-parse --short HEAD").trim()
                    DOCKER_IMAGE_TAG = "v${VERRAZZANO_DEV_VERSION}-${TIMESTAMP}-${SHORT_COMMIT_HASH}"
                }
            }
        }

        stage('Check Repo Clean') {
            steps {
                checkRepoClean()
                sh """
                   rm .trivyignore || true
                """
            }
        }

        stage('Build') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    make push DOCKER_IMAGE_NAME_OPERATOR=${DOCKER_IMAGE_NAME_OPERATOR} DOCKER_IMAGE_TAG=${DOCKER_IMAGE_TAG} K8S_NAMESPACE=${VMI_NAMESAPCE_PREFIX}-${env.BUILD_NUMBER} CREATE_LATEST_TAG=${CREATE_LATEST_TAG}
                """
            }
        }

        stage('golangci-lint Check') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    make golangci-lint
                """
            }
        }

        stage('Third Party License Check') {
            when { not { buildingTag() } }
            steps {
                thirdpartyCheck()
            }
        }

        stage('Copyright Compliance Check') {
            when { not { buildingTag() } }
            steps {
        	    copyrightScan "${WORKSPACE}"
            }
        }

        stage('Unit Tests') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    make unit-test
                    make -B coverage
                    cp coverage.html ${WORKSPACE}
                    build/scripts/copy-junit-output.sh ${WORKSPACE}
                """
            }
	        post {
                always {
                    archiveArtifacts artifacts: '**/coverage.html', allowEmptyArchive: true
                    junit testResults: '**/*test-result.xml', allowEmptyResults: true
                }
            }
        }

	    stage('Integration Tests') {
            when { not { buildingTag() } }
	        steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    echo "To do.."
                """
            }
        }

	    stage('basic1 integ tests oke') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    echo "To do.."
                """
            }
        }

	    stage('basic2 integ tests oke') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    echo "To do.."
                """
            }
        }

	    stage('basic3 integ tests oke') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    echo "To do.."
                """
            }
        }

	    stage('basic4 integ tests oke') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    echo "To do.."
                """
            }
        }

        stage('Scan Image') {
            when { not { buildingTag() } }
            steps {
                script {
                    scanContainerImage "${env.DOCKER_REPO}/${env.DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME_OPERATOR}:${DOCKER_IMAGE_TAG}"
                }
            }
            post {
                always {
                    archiveArtifacts artifacts: '**/*scanning-report.json', allowEmptyArchive: true
                }
            }
        }
    }

    post {
        failure {
            script {
                if (env.BRANCH_NAME == "master" || env.BRANCH_NAME ==~ "release-.*" || env.BRANCH_NAME ==~ "mark/*") {
                    slackSend ( message: "Job Failed - \"${env.JOB_NAME}\" build: ${env.BUILD_NUMBER}\n\nView the log at:\n ${env.BUILD_URL}\n\nBlue Ocean:\n${env.RUN_DISPLAY_URL}" )
                }
            }
        }
    }
}

def checkRepoClean() {
    sh """
        cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
        echo 'Check for forgotten manifest/generate actions...'
        (make check-repo-clean)
    """
}
