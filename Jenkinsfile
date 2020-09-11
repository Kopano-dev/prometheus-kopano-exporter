#!/usr/bin/env groovy

pipeline {
	agent {
		dockerfile {
			filename 'Dockerfile.build'
		 }
	}
	stages {
		stage('Bootstrap') {
			steps {
				echo 'Bootstrapping..'
				sh 'go version'
			}
		}
		stage('Lint') {
			steps {
				echo 'Linting..'
				sh 'make lint-checkstyle'
				recordIssues enabledForFailure: true, qualityGates: [[threshold: 100, type: 'TOTAL', unstable: true]], tools: [checkStyle(id: 'golint', name: 'Golint', pattern: 'test/tests.lint.xml')]
			}
		}
		stage('Vendor') {
			steps {
				echo 'Fetching vendor dependencies..'
				sh 'make vendor'
			}
		}
		stage('Build') {
			steps {
				echo 'Building..'
				sh 'make DATE=reproducible'
				sh './bin/prometheus-kopano-exporter version && sha256sum ./bin/prometheus-kopano-exporter'
			}
		}
		stage('Dist') {
			steps {
				echo 'Dist..'
				sh 'test -z "$(git diff --shortstat 2>/dev/null |tail -n1)" && echo "Clean check passed."'
				sh 'make check'
				sh 'make dist'
			}
		}
	}
	post {
		always {
			archiveArtifacts 'dist/*.tar.gz'
			cleanWs()
		}
	}
}
