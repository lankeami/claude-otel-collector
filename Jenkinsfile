#!/usr/bin/env groovy

pipeline {
  agent { label 'dind' }

  triggers {
    cron(BRANCH_NAME == "main" ? 'H 9 * * *' : '')
  }

  options {
    skipStagesAfterUnstable()
    buildDiscarder logRotator(artifactDaysToKeepStr: '5', artifactNumToKeepStr: '30', daysToKeepStr: '5', numToKeepStr: '30')
  }

  parameters {
    string(defaultValue: '095413357796', description: 'AWS Account ID', name: 'ACCOUNT_ID')
    string(defaultValue: "us-east-2", description: 'AWS Region', name: 'AWS_REGION')
    string(defaultValue: 'claude-otel-collector', description: 'ECR Registry Repos', name: 'REGISTRY_REPOS')
    string(defaultValue: ".", description: 'The source folder where the app is', name: "BASE_CONTEXT")
    string(defaultValue: "./Dockerfile", description: 'The Dockerfile path for building the image (relative to the BASE_CONTEXT)', name: 'DOCKERFILE_PATH')
  }

  stages {

    stage('Build setup') {
      steps {
        script {
          env = "B&B"
          account_id = params.ACCOUNT_ID
          registry_repos = params.REGISTRY_REPOS
          base_context = params.BASE_CONTEXT
          path_to_dockerfile = params.DOCKERFILE_PATH
          git_branch = scm.branches[0].name.replaceAll(/[\/\\]/, "-")
          build_number = currentBuild.number
          intermediate_tag = new Date().getTime()
          currentBuild.displayName = "#${currentBuild.number}: ${env} ${registry_repos} ${git_branch}"
          secret_name = (git_branch == "main" || git_branch == "master") ? "/prod/claude-otel-collector/env" : "/stg/claude-otel-collector/env"

          if (git_branch == 'main') {
              build_env = "production"
              helm_prefix = "main"
              build_env_short = "prod"
              prod_run = true
          } else {
              build_env = "staging"
              helm_prefix = "staging"
              build_env_short = "stg"
              prod_run = false
          }

          run_tag = "${account_id}.dkr.ecr.${AWS_REGION}.amazonaws.com/bollandbranch/${registry_repos}:${helm_prefix}-latest"
        }
      }
    }

    stage('Checkout Repository') {
      steps {
        checkout(
          [
            $class: 'GitSCM',
            branches: scm.branches,
            doGenerateSubmoduleConfigurations: false,
            extensions: [
              [
                $class: 'CloneOption',
                noTags: false,
                shallow: true,
                depth: 1
              ]
            ],
            submoduleCfg: [],
            userRemoteConfigs: [[credentialsId: 'github-ssh-access', url: "${GIT_URL}"]]
          ]
        )
      }
    }

    stage('Get Git SHA for Image name') {
      steps {
        script {
          sh "git rev-parse HEAD > .git/commit-id"
          git_commit_sha = readFile('.git/commit-id').trim()
        }
      }
    }

    stage('ECR Registry Login') {
      steps {
        script {
          sh """
            echo \$(aws ecr get-login-password --region ${AWS_REGION})|docker login --password-stdin --username AWS ${account_id}.dkr.ecr.${AWS_REGION}.amazonaws.com
          """
        }
      }
    }

    stage('Build Container') {
      steps {
        script {
          dir(base_context) {
            sh """
              aws --region=us-east-2 secretsmanager get-secret-value --secret-id ${secret_name} --query 'SecretString' --output text | jq -r "to_entries|map(\\"\\(.key)=\\(.value|tostring)\\")|.[]" >> .env
            """

            sh """
              docker build -t ${run_tag} -f ${path_to_dockerfile} .
            """
          }
        }
      }
    }

    stage('Push Container') {
      steps {
        script {
          dir(base_context) {
            sh """
              docker push ${run_tag}
              docker tag ${run_tag} ${account_id}.dkr.ecr.${AWS_REGION}.amazonaws.com/bollandbranch/${registry_repos}:${helm_prefix}-${build_number}-${git_commit_sha}
              docker push ${account_id}.dkr.ecr.${AWS_REGION}.amazonaws.com/bollandbranch/${registry_repos}:${helm_prefix}-${build_number}-${git_commit_sha}
            """
          }
        }
      }
    }

    stage('Update App Version in Helm chart') {
      steps {
        script {
          withCredentials([sshUserPrivateKey(credentialsId: 'github-ssh-access', keyFileVariable: 'ID_RSA_PATH', passphraseVariable: '', usernameVariable: 'USERNAME')]) {
            sh """
              eval `ssh-agent -s`
              ssh-add ${ID_RSA_PATH}
              cd deploy/
              git clone git@github.com:boll-branch/infrastructure.git

              rsync -avI --delete $registry_repos/ infrastructure/kubernetes/$build_env/charts/$registry_repos
              cd infrastructure/
              sed -i "s/^appVersion:.*\$/appVersion: \"${helm_prefix}-${build_number}-${git_commit_sha}\"/g" ./kubernetes/$build_env/charts/$registry_repos/Chart.yaml
              git --no-pager diff

              git config user.name BollandBranch
              git config user.email bot@bollandbranch.com
              git add .
              git commit -m "Jenkins - Argo Deploy $registry_repos $git_branch"
              git push -u origin master
            """
          }
        }
      }
    }

    stage('Argo Trigger') {
      when {
        anyOf {
          expression {
            return (GIT_BRANCH == 'master' || GIT_BRANCH == 'main')
          }
          expression {
            return GIT_BRANCH == 'staging'
          }
        }
      }
      steps {
        script {
          withCredentials([usernamePassword(credentialsId: 'argocd-credentials', passwordVariable: 'ARGOCD_PASSWORD', usernameVariable: 'ARGOCD_USERNAME')]) {
            sh """
            argocd login argocd.shared-services.bollandbranch.io --grpc-web --skip-test-tls --username $ARGOCD_USERNAME --password $ARGOCD_PASSWORD
            argocd app get $build_env-$registry_repos --hard-refresh
            argocd app wait $build_env-$registry_repos --timeout 300
            """
          }
        }
      }
    }

    stage('Send Build End Notification') {
      steps {
        script {
          slackSend channel: '#backend-deployments',
            color: 'good',
            message: "`claude-otel-collector` Build Complete [`${git_branch.toUpperCase()}`][<${BUILD_URL}|#${build_number}>]",
            teamDomain: 'bollandbranch',
            tokenCredentialId: 'deploybot-slack-credentials',
            username: "deploybot",
            botUser: "false"
        }
      }
    }

  }
  post {
    failure {
      script {
        slackSend channel: '#backend-deployments',
          color: 'danger',
          message: "`claude-otel-collector` Build Failure [`${git_branch.toUpperCase()}`][<${BUILD_URL}|#${build_number}>]",
          teamDomain: 'bollandbranch',
          tokenCredentialId: 'deploybot-slack-credentials',
          username: "deploybot",
          botUser: "false"
      }
    }
  }
}
