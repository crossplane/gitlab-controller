pipeline {
    agent { label 'upbound-gce' }

    options {
        disableConcurrentBuilds()
        timestamps()
    }

    environment {
        RUNNING_IN_CI = 'true'
        REPOSITORY_NAME = "${env.GIT_URL.tokenize('/')[3].split('\\.')[0]}"
        REPOSITORY_OWNER = "${env.GIT_URL.tokenize('/')[2]}"
        DOCKER = credentials('dockerhub-upboundci')
        AWS = credentials('aws-upbound-bot')
        GITHUB_UPBOUND_BOT = credentials('github-upbound-jenkins')
    }

    stages {

        stage('Prepare') {
            steps {
                sh 'git config --global user.name "upbound-bot"'
                sh 'echo "machine github.com login upbound-bot password $GITHUB_UPBOUND_BOT" > ~/.netrc'
            }
        }

        stage('Build'){
            steps {
                sh './build/run make vendor.check'
                sh './build/run make -j\$(nproc) build.all'
            }
            post {
                always {
                    archiveArtifacts "_output/lint/**/*"
                    ViolationsToGitHub([
                        gitHubUrl: env.GIT_URL,
                        repositoryName: env.REPOSITORY_NAME,
                        repositoryOwner: env.REPOSITORY_OWNER,
                        pullRequestId: env.CHANGE_ID,
                        oAuth2Token: env.GITHUB_UPBOUND_BOT_PSW,

                        createCommentWithAllSingleFileComments: false,
                        createSingleFileComments: true,
                        keepOldComments: false,
                        commentOnlyChangedContent: true,
                        commentTemplate: readFile('hack/linter-violation.tmpl'),

                        violationConfigs: [[
                            reporter: 'make lint',
                            parser: 'CHECKSTYLE',
                            // This is a regex run against the absolute path of the file.
                            pattern: '.*/_output/lint/.+/checkstyle\\.xml\$',
                        ]]
                    ])
                }
            }
        }

        stage('Unit Tests') {
            steps {
                sh './build/run make -j\$(nproc) test'
                sh './build/run make -j\$(nproc) cobertura'
            }
            post {
                always {
                    archiveArtifacts "_output/tests/**/*"
                    junit "_output/tests/**/unit-tests.xml"
                    cobertura coberturaReportFile: '_output/tests/**/cobertura-coverage.xml',
                            classCoverageTargets: '50, 0, 0',
                            conditionalCoverageTargets: '70, 0, 0',
                            lineCoverageTargets: '40, 0, 0',
                            methodCoverageTargets: '30, 0, 0',
                            packageCoverageTargets: '80, 0, 0',
                            autoUpdateHealth: false,
                            autoUpdateStability: false,
                            enableNewApi: false,
                            failUnhealthy: false,
                            failUnstable: false,
                            maxNumberOfBuilds: 0,
                            onlyStable: false,
                            sourceEncoding: 'ASCII',
                            zoomCoverageChart: false
                }
            }
        }

        stage('SonarQube Analysis') {
            steps {
                script {
                    scannerHome = tool 'SonarQubeScanner'
                    scannerParams = ''
                    if (env.CHANGE_ID == null) {
                        scannerParams = "-Dsonar.branch.name=${BRANCH_NAME} "
                        if (BRANCH_NAME != 'master') {
                            scannerParams = "${scannerParams} -Dsonar.branch.target=master"
                        }
                    } else {
                        scannerParams = "-Dsonar.pullrequest.base=master " +
                            "-Dsonar.pullrequest.branch=${env.BRANCH_NAME} " +
                            "-Dsonar.pullrequest.key=${env.CHANGE_ID}  " +
                            "-Dsonar.pullrequest.provider=github " +
                            "-Dsonar.pullrequest.github.repository=crossplaneio/${env.REPOSITORY_NAME}"
                    }
                }

                withSonarQubeEnv('SonarQubeCrossplane') {
                  sh "${scannerHome}/bin/sonar-scanner " +
                    "-Dsonar.projectKey=${env.REPOSITORY_NAME} " +
                    "-Dsonar.projectName=${env.REPOSITORY_NAME} " +
                    "-Dsonar.organization=crossplane " +
                    "-Dsonar.sources=. ${scannerParams} "
                }
            }
        }

        stage('Record Coverage') {
            when { branch 'master' }
            steps {
                script {
                    currentBuild.result = 'SUCCESS'
                 }
                step([$class: 'MasterCoverageAction', scmVars: [GIT_URL: env.GIT_URL]])
            }
        }

        stage('PR Coverage to Github') {
            when { allOf {not { branch 'master' }; expression { return env.CHANGE_ID != null }} }
            steps {
                script {
                    currentBuild.result = 'SUCCESS'
                }
                step([$class: 'CompareCoverageAction', publishResultAs: 'comment', scmVars: [GIT_URL: env.GIT_URL]])
            }
        }

        stage('Publish') {
            steps {
                sh 'docker login -u="${DOCKER_USR}" -p="${DOCKER_PSW}"'
                sh "./build/run make -j\$(nproc) publish BRANCH_NAME=${BRANCH_NAME} AWS_ACCESS_KEY_ID=${AWS_USR} AWS_SECRET_ACCESS_KEY=${AWS_PSW} GIT_API_TOKEN=${GITHUB_UPBOUND_BOT}"
                script {
                    if (BRANCH_NAME == 'master') {
                        lock('promote-job') {
                            sh "./build/run make -j\$(nproc) promote BRANCH_NAME=master CHANNEL=master AWS_ACCESS_KEY_ID=${AWS_USR} AWS_SECRET_ACCESS_KEY=${AWS_PSW}"
                        }
                    }
                }
            }
        }
    }

    post {
        always {
            script {
                sh 'make -j\$(nproc) clean'
                sh 'make -j\$(nproc) prune PRUNE_HOURS=48 PRUNE_KEEP=48'
                sh 'docker images'
                deleteDir()
            }
        }
    }
}
