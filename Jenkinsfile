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
				checkstyle pattern: 'test/tests.lint.xml', canComputeNew: false, unstableTotalHigh: '100'
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
				sh './bin/promtheus-kopano-exporter version && sha256sum ./bin/prometheus-kopano-exporter'
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
