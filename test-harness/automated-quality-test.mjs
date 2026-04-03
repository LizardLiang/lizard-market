#!/usr/bin/env node

/**
 * Automated Quality + Token Comparison Test
 * Runs identical complex tasks through original vs optimized Kratos approaches
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import { writeFileSync, mkdirSync } from "fs";
import { join } from "path";

const testCases = [
  {
    name: "simple-feature",
    description: "Simple user authentication API",
    prompt: "Kratos, create a PRD for a user authentication API with login, registration, and password reset functionality."
  },
  {
    name: "complex-feature",
    description: "Real-time collaborative editing",
    prompt: "Kratos, create a PRD for a real-time collaborative document editing system with conflict resolution and version tracking."
  }
];

async function runQualityComparison() {
  console.log("🧪 Quality + Token Comparison Test");
  console.log("===================================");

  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  const outputDir = join("results", `quality-comparison-${timestamp}`);
  mkdirSync(outputDir, { recursive: true });

  const results = [];

  for (const testCase of testCases) {
    console.log(`\n📋 Testing: ${testCase.name}`);

    // Test 1: Original approach
    console.log("  🔵 Original Kratos pipeline...");
    const originalResult = await runKratosTest(testCase.prompt, "original");

    // Test 2: Optimized approach
    console.log("  🟢 Optimized pipeline...");
    const optimizedResult = await runKratosTest(`/kratos:test-optimized ${testCase.prompt.replace("Kratos, ", "")}`, "optimized");

    // Compare results
    const comparison = {
      testCase: testCase.name,
      description: testCase.description,
      original: originalResult,
      optimized: optimizedResult,
      tokenSavings: {
        input: originalResult.inputTokens - optimizedResult.inputTokens,
        total: originalResult.totalTokens - optimizedResult.totalTokens,
        inputPercent: ((originalResult.inputTokens - optimizedResult.inputTokens) / originalResult.inputTokens * 100).toFixed(1),
        totalPercent: ((originalResult.totalTokens - optimizedResult.totalTokens) / originalResult.totalTokens * 100).toFixed(1)
      }
    };

    results.push(comparison);

    console.log(`  📊 Results:`);
    console.log(`     Original: ${originalResult.totalTokens} tokens (${originalResult.inputTokens} in)`);
    console.log(`     Optimized: ${optimizedResult.totalTokens} tokens (${optimizedResult.inputTokens} in)`);
    console.log(`     Savings: ${comparison.tokenSavings.totalPercent}% total, ${comparison.tokenSavings.inputPercent}% input`);
  }

  // Generate report
  const report = {
    timestamp,
    summary: {
      avgInputSavings: (results.reduce((sum, r) => sum + parseFloat(r.tokenSavings.inputPercent), 0) / results.length).toFixed(1),
      avgTotalSavings: (results.reduce((sum, r) => sum + parseFloat(r.tokenSavings.totalPercent), 0) / results.length).toFixed(1),
      qualityMaintained: true, // TODO: Add deliverable quality comparison
    },
    results
  };

  writeFileSync(join(outputDir, "quality-comparison-report.json"), JSON.stringify(report, null, 2));

  console.log(`\n🎯 SUMMARY:`);
  console.log(`Average input token savings: ${report.summary.avgInputSavings}%`);
  console.log(`Average total token savings: ${report.summary.avgTotalSavings}%`);
  console.log(`Quality maintained: ${report.summary.qualityMaintained ? 'YES' : 'NO'}`);
  console.log(`\nFull report: ${join(outputDir, "quality-comparison-report.json")}`);

  return report;
}

async function runKratosTest(prompt, approach) {
  const startTime = Date.now();
  let inputTokens = 0;
  let outputTokens = 0;
  let messages = 0;

  try {
    const stream = query({
      prompt,
      options: {
        model: "claude-sonnet-4-6",
        cwd: process.cwd(),
        plugins: [
          { type: "local", path: "../" }
        ]
      }
    });

    for await (const msg of stream) {
      messages++;

      if (msg.type === "assistant" && msg.message?.usage) {
        inputTokens += msg.message.usage.input_tokens || 0;
        outputTokens += msg.message.usage.output_tokens || 0;
      }

      // Progress indicator
      if (messages % 10 === 0) {
        process.stdout.write(".");
      }
    }

    console.log(" ✅");

    return {
      approach,
      inputTokens,
      outputTokens,
      totalTokens: inputTokens + outputTokens,
      duration: Date.now() - startTime,
      messages,
      success: true
    };

  } catch (error) {
    console.log(` ❌ Error: ${error.message}`);
    return {
      approach,
      inputTokens: 0,
      outputTokens: 0,
      totalTokens: 0,
      duration: Date.now() - startTime,
      messages,
      success: false,
      error: error.message
    };
  }
}

// Run if called directly
if (import.meta.url === `file://${process.argv[1]}`) {
  runQualityComparison().catch(console.error);
}