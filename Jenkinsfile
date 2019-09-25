pipeline {
    agent {
        node {
            label 'docker'
        }
    }

    environment {
        SSHAGENT_CREDENTIALS_ID = '325da657-87ef-403e-aadb-cef3b0940612'
        REGISTRY_CREDENTIALS_ID = 'artifactory'
        REGISTRY_URL = 'https://linode-docker.artifactory.linode.com/'
        IMAGE_NAME = 'linode-docker.artifactory.linode.com/lke/cluster-api-provider-lke'
        GIT_COMMIT_12 = "${GIT_COMMIT}".substring(0, 12)
        BUILD_IMAGE_TAG = "${GIT_COMMIT_12}-${BUILD_NUMBER}"
        // IMG is used in the Makefile to determine the name of the manager image to build
        IMG = "${IMAGE_NAME}:${BUILD_IMAGE_TAG}"
        // IMG is used in the Makefile to determine the name of the source copy image to build
        SRC_IMG = "${BUILD_IMAGE_TAG}-source"
    }

    options {
        parallelsAlwaysFailFast()
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        // Lint: Lint the code
        stage('Lint') {
            steps {
                sshagent (credentials: [env.SSHAGENT_CREDENTIALS_ID]) {
                    sh "make lint"
                    sh "make vet"
                }
            }
        }

        // Build: Build (and test) CAPLKE
        stage('Build') {
            parallel {
                // Test: Run tests against the CAPLKE source
                stage('Test') {
                    steps {
                        sshagent (credentials: [env.SSHAGENT_CREDENTIALS_ID]) {
                            sh "make -j4 test"
                        }
                    }
                }

                // Build: Build the runnable Docker image
                stage('Build') {
                    steps {
                        sshagent (credentials: [env.SSHAGENT_CREDENTIALS_ID]) {
                            sh "CAPLKE_REVISION=${GIT_COMMIT_12} make -j4 docker-build"
                        }
                    }
                }
            }
        }

        // Push: Push Docker images up to Artifactory
        stage('Push') {
            parallel {
                // Push master: Tag and push the master image for CAPLKE
                stage('Push master') {
                    when {
                        branch 'master'
                    }

                    steps {
                        script {
                            docker.withRegistry(env.REGISTRY_URL, env.REGISTRY_CREDENTIALS_ID) {
                                sh "docker tag ${IMG} ${IMAGE_NAME}:master"
                                sh "docker push ${IMAGE_NAME}:master"
                            }
                        }
                    }
                }
                // Push latest: Tag and push the latest image for CAPLKE
                stage('Push latest') {
                    when {
                        branch 'master'
                        tag 'v*'
                    }

                    steps {
                        script {
                            docker.withRegistry(env.REGISTRY_URL, env.REGISTRY_CREDENTIALS_ID) {
                                sh "docker tag ${IMG} ${IMAGE_NAME}:latest"
                                sh "docker tag ${IMG} ${IMAGE_NAME}:${TAG_NAME}"
                                sh "docker push ${IMAGE_NAME}:latest ${IMAGE_NAME}:${TAG_NAME}"
                            }
                        }
                    }
                }
            }
        }
    }

    post {
        // Cleanup: Remove the images we created (dangling layers will be saved unless a prune operation happens)
        cleanup {
            // If the images fail to be deleted for some reason, don't make the whole job fail
            sh "docker image rm ${IMG} ${SRC_IMG} || exit 0"
            deleteDir()
        }
    }
}
