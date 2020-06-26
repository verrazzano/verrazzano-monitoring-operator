// Copyright (c) 2020, Oracle Corporation and/or its affiliates. 
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

def HEAD_COMMIT

pipeline {
    options {
        skipDefaultCheckout true
        disableConcurrentBuilds()
    }

    agent {
       docker {
            image "${RUNNER_DOCKER_IMAGE}"
            args "${RUNNER_DOCKER_ARGS}"
            registryUrl "${RUNNER_DOCKER_REGISTRY_URL}"
            registryCredentialsId 'ocir-pull-and-push-account'
       }
    }

    environment {
        DOCKER_CI_IMAGE_NAME = 'verrazzano-monitoring-operator-jenkins'
        DOCKER_PUBLISH_IMAGE_NAME = 'verrazzano-monitoring-operator'
        DOCKER_IMAGE_NAME = "${env.BRANCH_NAME == 'master' ? env.DOCKER_PUBLISH_IMAGE_NAME : env.DOCKER_CI_IMAGE_NAME}"
        CREATE_LATEST_TAG = "${env.BRANCH_NAME == 'master' ? '1' : '0'}"
        GOPATH = '/home/opc/go'
        GO_REPO_PATH = "${GOPATH}/src/github.com/verrazzano"
        DOCKER_CREDS = credentials('ocir-pull-and-push-account')
        HELM_CHART_NAME = 'verrazzano-monitoring-operator'
        VMI_NAMESAPCE_PREFIX = 'vmi'
        ELASTICSEARCH_VERSION = '7.2.0'
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
            }
        }
       
        stage('Build') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    make push DOCKER_IMAGE_NAME=${DOCKER_IMAGE_NAME} K8S_NAMESPACE=${VMI_NAMESAPCE_PREFIX}-${env.BUILD_NUMBER} CREATE_LATEST_TAG=${CREATE_LATEST_TAG}
                """
            }
        }

        stage('Third Party License Check') {
            when { not { buildingTag() } }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    make thirdparty-check
                """
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
                    HEAD_COMMIT = sh(returnStdout: true, script: "git rev-parse HEAD").trim()
                    clairScanTemp "${env.DOCKER_REPO}/${env.DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME}:${HEAD_COMMIT}"
                }
            }
            post {
                always {
                    archiveArtifacts artifacts: '**/scanning-report.json', allowEmptyArchive: true
                }
            }
        }

        stage('Publish Image') {
            when { buildingTag() }
            steps {
                sh """
                    cd ${GO_REPO_PATH}/verrazzano-monitoring-operator
                    make push-tag DOCKER_REPO=${env.DOCKER_REPO} DOCKER_NAMESPACE=${env.DOCKER_NAMESPACE} DOCKER_IMAGE_NAME=${env.DOCKER_PUBLISH_IMAGE_NAME}
                    make k8s-dist DOCKER_IMAGE_NAME=${DOCKER_PUBLISH_IMAGE_NAME} VERSION=${BRANCH_NAME} K8S_NAMESPACE=default
                """
            }
        }
    }

    post {
        failure {
            mail to: "${env.BUILD_NOTIFICATION_TO_EMAIL}", from: 'noreply@oracle.com',
            subject: "Verrazzano: ${env.JOB_NAME} - Failed",
            body: "Job Failed - \"${env.JOB_NAME}\" build: ${env.BUILD_NUMBER}\n\nView the log at:\n ${env.BUILD_URL}\n\nBlue Ocean:\n${env.RUN_DISPLAY_URL}"
        }
    }
    
}
