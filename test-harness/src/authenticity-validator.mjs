#!/usr/bin/env node
/**
 * Kratos Timestamp Authenticity Validator
 *
 * Validates that agents are using REAL timestamps instead of making up times.
 * Checks for suspicious patterns that indicate fabricated timestamps.
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

/**
 * Validate timestamp authenticity (not fabricated)
 */
function validateTimestampAuthenticity(statusData, testStartTime, testEndTime) {
  const errors = [];
  const warnings = [];
  const info = [];

  // Convert test bounds to Date objects
  const testStart = testStartTime ? new Date(testStartTime) : null;
  const testEnd = testEndTime ? new Date(testEndTime) : null;

  // Check created timestamp authenticity
  if (statusData.created) {
    const createdTime = new Date(statusData.created);

    // Check if created time is within reasonable test window
    if (testStart && testEnd) {
      if (createdTime < testStart) {
        const beforeMs = testStart.getTime() - createdTime.getTime();
        if (beforeMs > 60000) { // More than 1 minute before test
          warnings.push(`Created timestamp is ${Math.round(beforeMs/1000)}s before test start - may be stale data`);
        }
      }

      if (createdTime > testEnd) {
        const afterMs = createdTime.getTime() - testEnd.getTime();
        if (afterMs > 60000) { // More than 1 minute after test
          errors.push(`Created timestamp is ${Math.round(afterMs/1000)}s after test end - likely fabricated`);
        }
      }

      // Check if created time is within test window
      const inWindow = createdTime >= testStart && createdTime <= testEnd;
      if (inWindow) {
        info.push('Created timestamp is within test execution window ✓');
      }
    }
  }

  const stagesData = statusData.stages || statusData.pipeline;
  if (!stagesData) {
    errors.push("No stages or pipeline data found");
    return { errors, warnings, info };
  }

  const authenticityChecks = [];

  // Analyze each stage for timestamp authenticity
  Object.keys(stagesData).sort().forEach(stageName => {
    const stage = stagesData[stageName];

    if (!stage.started && !stage.completed) return; // Skip stages with no timing data

    const check = {
      stage: stageName,
      status: stage.status,
      authentic: true,
      issues: []
    };

    if (stage.started) {
      const startTime = new Date(stage.started);

      // Check if started time is within test window
      if (testStart && testEnd) {
        if (startTime < testStart || startTime > testEnd) {
          const outside = startTime < testStart ?
            `${Math.round((testStart.getTime() - startTime.getTime())/1000)}s before` :
            `${Math.round((startTime.getTime() - testEnd.getTime())/1000)}s after`;
          check.issues.push(`Started timestamp is ${outside} test window`);
          check.authentic = false;
        }
      }

      // Check for suspiciously round timestamps (common fabrication pattern)
      const minutes = startTime.getMinutes();
      const seconds = startTime.getSeconds();
      if (minutes === 0 && seconds === 0) {
        check.issues.push('Started at exactly top of hour (suspicious pattern)');
        warnings.push(`Stage ${stageName}: Started at suspiciously round time`);
      }
    }

    if (stage.completed) {
      const completeTime = new Date(stage.completed);

      // Check if completed time is within test window
      if (testStart && testEnd) {
        if (completeTime < testStart || completeTime > testEnd) {
          const outside = completeTime < testStart ?
            `${Math.round((testStart.getTime() - completeTime.getTime())/1000)}s before` :
            `${Math.round((completeTime.getTime() - testEnd.getTime())/1000)}s after`;
          check.issues.push(`Completed timestamp is ${outside} test window`);
          check.authentic = false;
        }
      }

      // Check for suspiciously round timestamps
      const minutes = completeTime.getMinutes();
      const seconds = completeTime.getSeconds();
      if (minutes === 0 && seconds === 0) {
        check.issues.push('Completed at exactly top of hour (suspicious pattern)');
        warnings.push(`Stage ${stageName}: Completed at suspiciously round time`);
      }
    }

    // Check for impossible rapid completion patterns
    if (stage.started && stage.completed) {
      const startTime = new Date(stage.started);
      const completeTime = new Date(stage.completed);
      const duration = completeTime.getTime() - startTime.getTime();

      // Flag suspiciously fast completions that might be fabricated
      if (stageName.includes('prd') && duration < 5000) {
        check.issues.push(`PRD work completed in ${duration}ms (< 5s) - suspiciously fast`);
        warnings.push(`Stage ${stageName}: Unrealistically fast completion`);
      }

      if (stageName.includes('spec') && duration < 10000) {
        check.issues.push(`Spec work completed in ${duration}ms (< 10s) - suspiciously fast`);
        warnings.push(`Stage ${stageName}: Unrealistically fast completion`);
      }

      if (stageName.includes('implementation') && duration < 30000) {
        check.issues.push(`Implementation completed in ${duration}ms (< 30s) - suspiciously fast`);
        warnings.push(`Stage ${stageName}: Unrealistically fast completion`);
      }
    }

    // Check timestamp sequence authenticity
    if (stage.started && stage.completed) {
      const startTime = new Date(stage.started);
      const completeTime = new Date(stage.completed);

      // Check for identical timestamps (often indicates fabrication)
      if (startTime.getTime() === completeTime.getTime()) {
        if (stageName.includes('review') || stageName.includes('spec')) {
          errors.push(`Stage ${stageName}: Started and completed at identical time - likely fabricated`);
          check.authentic = false;
          check.issues.push('Identical start/complete timestamps');
        }
      }
    }

    authenticityChecks.push(check);
  });

  // Cross-reference with history timestamps if available
  if (statusData.history) {
    statusData.history.forEach(entry => {
      if (entry.timestamp && testStart && testEnd) {
        const historyTime = new Date(entry.timestamp);
        if (historyTime < testStart || historyTime > testEnd) {
          warnings.push(`History entry timestamp outside test window: ${entry.action}`);
        }
      }
    });
  }

  // Pattern analysis for fabrication detection
  const allTimestamps = [];
  if (statusData.created) allTimestamps.push(new Date(statusData.created));
  if (statusData.updated) allTimestamps.push(new Date(statusData.updated));

  Object.values(stagesData).forEach(stage => {
    if (stage.started) allTimestamps.push(new Date(stage.started));
    if (stage.completed) allTimestamps.push(new Date(stage.completed));
  });

  // Check for suspicious timestamp patterns
  if (allTimestamps.length >= 3) {
    const intervals = [];
    for (let i = 1; i < allTimestamps.length; i++) {
      const interval = allTimestamps[i].getTime() - allTimestamps[i-1].getTime();
      intervals.push(interval);
    }

    // Check for too-regular intervals (fabrication pattern)
    const avgInterval = intervals.reduce((sum, val) => sum + val, 0) / intervals.length;
    const regularCount = intervals.filter(interval => Math.abs(interval - avgInterval) < 1000).length;

    if (regularCount >= intervals.length * 0.8) {
      warnings.push('Timestamps show suspiciously regular intervals - may be fabricated');
    }
  }

  return {
    errors,
    warnings,
    info,
    authenticityChecks,
    authenticityRate: authenticityChecks.length > 0 ?
      Math.round((authenticityChecks.filter(c => c.authentic).length / authenticityChecks.length) * 100) : 100
  };
}

/**
 * Analyze test result for timestamp authenticity
 */
function analyzeAuthenticityResult(resultDir) {
  const analysis = {
    testName: path.basename(resultDir),
    statusJsonFound: false,
    authenticityValidation: null,
    featurePath: null,
    testTiming: null
  };

  try {
    // Get test timing information
    const summaryPath = path.join(resultDir, 'summary.json');
    if (fs.existsSync(summaryPath)) {
      const summaryData = JSON.parse(fs.readFileSync(summaryPath, 'utf8'));
      analysis.testTiming = {
        startedAt: summaryData.startedAt,
        finishedAt: summaryData.finishedAt
      };
    }

    // Look for status.json files in the test project
    const testProjectDir = path.join(resultDir, '_test-project');
    if (!fs.existsSync(testProjectDir)) {
      analysis.error = 'Test project directory not found';
      return analysis;
    }

    const claudeDir = path.join(testProjectDir, '.claude');
    if (!fs.existsSync(claudeDir)) {
      analysis.error = 'No .claude directory found in test project';
      return analysis;
    }

    const featureDir = path.join(claudeDir, 'feature');
    if (!fs.existsSync(featureDir)) {
      analysis.error = 'No .claude/feature directory found';
      return analysis;
    }

    // Look for any feature subdirectories with status.json
    const features = fs.readdirSync(featureDir, { withFileTypes: true })
      .filter(dirent => dirent.isDirectory())
      .map(dirent => dirent.name);

    for (const feature of features) {
      const statusJsonPath = path.join(featureDir, feature, 'status.json');
      if (fs.existsSync(statusJsonPath)) {
        analysis.statusJsonFound = true;
        analysis.featurePath = path.join('feature', feature);

        try {
          const statusContent = fs.readFileSync(statusJsonPath, 'utf8');
          const statusData = JSON.parse(statusContent);
          analysis.authenticityValidation = validateTimestampAuthenticity(
            statusData,
            analysis.testTiming?.startedAt,
            analysis.testTiming?.finishedAt
          );
          analysis.statusData = statusData;
        } catch (parseError) {
          analysis.error = `Failed to parse status.json: ${parseError.message}`;
        }
        break;
      }
    }

    if (!analysis.statusJsonFound) {
      analysis.error = 'No status.json files found in any feature directory';
    }

  } catch (error) {
    analysis.error = `Analysis failed: ${error.message}`;
  }

  return analysis;
}

/**
 * Run authenticity validation tests
 */
export async function runAuthenticityValidation(resultsDir) {
  console.log('🔍 Kratos Timestamp Authenticity Validation');
  console.log('==========================================');
  console.log('');
  console.log('Checking that agents use REAL timestamps instead of fabricated ones...');
  console.log('');

  if (!fs.existsSync(resultsDir)) {
    console.error(`❌ Results directory not found: ${resultsDir}`);
    return false;
  }

  const testDirs = fs.readdirSync(resultsDir, { withFileTypes: true })
    .filter(dirent => dirent.isDirectory() && dirent.name.match(/^\d{4}-\d{2}-\d{2}T/))
    .map(dirent => dirent.name)
    .sort()
    .reverse(); // Most recent first

  if (testDirs.length === 0) {
    console.error('❌ No test result directories found');
    return false;
  }

  let totalTests = 0;
  let authenticTests = 0;
  let fabricatedTests = 0;

  console.log(`Found ${testDirs.length} test run(s)\n`);

  for (const testDir of testDirs) {
    const fullPath = path.join(resultsDir, testDir);
    console.log(`📁 Analyzing: ${testDir}`);

    // Find timestamp-related tests in this run
    const taskDirs = fs.readdirSync(fullPath, { withFileTypes: true })
      .filter(dirent => dirent.isDirectory() && dirent.name.startsWith('timestamp-'))
      .map(dirent => dirent.name);

    if (taskDirs.length === 0) {
      console.log('   ⏭️  No timestamp tests found in this run\n');
      continue;
    }

    for (const taskName of taskDirs) {
      totalTests++;
      const taskPath = path.join(fullPath, taskName);
      console.log(`   🔍 Testing: ${taskName}`);

      const analysis = analyzeAuthenticityResult(taskPath);

      if (analysis.error) {
        console.log(`      ❌ ${analysis.error}`);
        fabricatedTests++;
        continue;
      }

      if (!analysis.statusJsonFound) {
        console.log('      ⚠️  No status.json created (test may not have triggered status updates)');
        continue;
      }

      if (!analysis.authenticityValidation) {
        console.log('      ❌ Authenticity validation failed');
        fabricatedTests++;
        continue;
      }

      const { errors, warnings, info, authenticityChecks, authenticityRate } = analysis.authenticityValidation;

      if (errors.length === 0) {
        console.log('      ✅ Timestamps appear authentic');
        authenticTests++;

        console.log(`      🎯 Authenticity rate: ${authenticityRate}%`);

        // Show authenticity checks
        if (authenticityChecks.length > 0) {
          console.log('      📊 Stage authenticity:');
          authenticityChecks.forEach(check => {
            const status = check.authentic ? '✅' : '❌';
            const issues = check.issues.length > 0 ? ` (${check.issues.join(', ')})` : '';
            console.log(`         ${check.stage}: ${status}${issues}`);
          });
        }

        if (warnings.length > 0) {
          console.log('      ⚠️  Authenticity warnings:');
          warnings.forEach(warning => console.log(`         - ${warning}`));
        }

        if (info.length > 0) {
          console.log('      💡 Authenticity info:');
          info.forEach(i => console.log(`         - ${i}`));
        }
      } else {
        console.log('      ❌ Timestamp authenticity errors:');
        errors.forEach(error => console.log(`         - ${error}`));

        if (warnings.length > 0) {
          console.log('      ⚠️  Authenticity warnings:');
          warnings.forEach(warning => console.log(`         - ${warning}`));
        }
        fabricatedTests++;
      }

      // Show test timing context
      if (analysis.testTiming) {
        const testStart = new Date(analysis.testTiming.startedAt);
        const testEnd = new Date(analysis.testTiming.finishedAt);
        const duration = Math.round((testEnd.getTime() - testStart.getTime()) / 1000);
        console.log(`      🕐 Test window: ${duration}s (${testStart.toLocaleTimeString()} - ${testEnd.toLocaleTimeString()})`);
      }
    }

    console.log('');
  }

  // Summary
  console.log('🔍 Authenticity Summary');
  console.log('======================');
  console.log(`Total tests: ${totalTests}`);
  console.log(`✅ Authentic: ${authenticTests}`);
  console.log(`❌ Fabricated/Suspicious: ${fabricatedTests}`);

  if (totalTests === 0) {
    console.log('⚠️  No timestamp tests found. Run timestamp tests first.');
    return false;
  }

  const authenticityRate = Math.round((authenticTests / totalTests) * 100);
  console.log(`🎯 Authenticity rate: ${authenticityRate}%`);

  return fabricatedTests === 0;
}

// CLI interface
if (import.meta.url === `file://${process.argv[1]}`) {
  const resultsDir = process.argv[2] || path.join(__dirname, '..', 'results');
  runAuthenticityValidation(resultsDir)
    .then(success => process.exit(success ? 0 : 1))
    .catch(error => {
      console.error('❌ Authenticity validation failed:', error.message);
      process.exit(1);
    });
}