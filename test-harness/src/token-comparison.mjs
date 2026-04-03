#!/usr/bin/env node

/**
 * Token Optimization Test Script
 * Runs comparison tests between original and optimized Kratos approaches
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import { writeFileSync, mkdirSync } from "fs";
import { join } from "path";

const testTasks = [
  {
    name: "prd-comparison",
    original: "Kratos, create a PRD for a task management API with user authentication, task CRUD, and team collaboration features.",
    optimized: "/kratos:test-optimized Create a PRD for a task management API with user authentication, task CRUD, and team collaboration features."
  },
  {
    name: "spec-comparison",
    original: "Kratos, create a technical specification for a file upload service with virus scanning, storage optimization, and CDN integration.",
    optimized: "/kratos:test-optimized Create a technical specification for a file upload service with virus scanning, storage optimization, and CDN integration."
  }
];

async function runComparison() {
  console.log("🧪 Token Optimization Comparison Test");
  console.log("=====================================");

  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  const outputDir = join("results", `token-comparison-${timestamp}`);
  mkdirSync(outputDir, { recursive: true });

  const results = [];

  for (const task of testTasks) {
    console.log(`\n📊 Testing: ${task.name}`);

    // Test original approach
    console.log("  🔵 Original approach...");
    const originalResult = await runSingleTest(task.original, "original");

    // Test optimized approach
    console.log("  🟢 Optimized approach...");
    const optimizedResult = await runSingleTest(task.optimized, "optimized");

    // Compare results
    const comparison = {
      task: task.name,
      original: {
        tokens: originalResult.inputTokens + originalResult.outputTokens,
        inputTokens: originalResult.inputTokens,
        outputTokens: originalResult.outputTokens,
        duration: originalResult.duration,
        messages: originalResult.messages
      },
      optimized: {
        tokens: optimizedResult.inputTokens + optimizedResult.outputTokens,
        inputTokens: optimizedResult.inputTokens,
        outputTokens: optimizedResult.outputTokens,
        duration: optimizedResult.duration,
        messages: optimizedResult.messages
      }
    };

    // Calculate savings
    comparison.tokenSavings = {
      absolute: comparison.original.tokens - comparison.optimized.tokens,
      percentage: ((comparison.original.tokens - comparison.optimized.tokens) / comparison.original.tokens * 100).toFixed(1)
    };

    comparison.inputTokenSavings = {
      absolute: comparison.original.inputTokens - comparison.optimized.inputTokens,
      percentage: ((comparison.original.inputTokens - comparison.optimized.inputTokens) / comparison.original.inputTokens * 100).toFixed(1)
    };

    results.push(comparison);

    console.log(`  📈 Results:`);
    console.log(`     Original: ${comparison.original.tokens} tokens (${comparison.original.inputTokens} in, ${comparison.original.outputTokens} out)`);
    console.log(`     Optimized: ${comparison.optimized.tokens} tokens (${comparison.optimized.inputTokens} in, ${comparison.optimized.outputTokens} out)`);
    console.log(`     Savings: ${comparison.tokenSavings.absolute} tokens (${comparison.tokenSavings.percentage}%)`);
    console.log(`     Input Savings: ${comparison.inputTokenSavings.absolute} tokens (${comparison.inputTokenSavings.percentage}%)`);
  }

  // Generate summary report
  const report = {
    timestamp,
    summary: {
      totalTests: results.length,
      averageTokenSavings: (results.reduce((sum, r) => sum + parseFloat(r.tokenSavings.percentage), 0) / results.length).toFixed(1),
      averageInputSavings: (results.reduce((sum, r) => sum + parseFloat(r.inputTokenSavings.percentage), 0) / results.length).toFixed(1),
      maxSavings: Math.max(...results.map(r => parseFloat(r.tokenSavings.percentage))).toFixed(1),
      minSavings: Math.min(...results.map(r => parseFloat(r.tokenSavings.percentage))).toFixed(1)
    },
    results
  };

  // Save results
  writeFileSync(join(outputDir, "comparison-report.json"), JSON.stringify(report, null, 2));

  console.log(`\n📊 SUMMARY`);
  console.log(`==========`);
  console.log(`Tests completed: ${report.summary.totalTests}`);
  console.log(`Average token savings: ${report.summary.averageTokenSavings}%`);
  console.log(`Average input token savings: ${report.summary.averageInputSavings}%`);
  console.log(`Range: ${report.summary.minSavings}% - ${report.summary.maxSavings}%`);
  console.log(`\nDetailed results saved to: ${outputDir}`);

  return report;
}

async function runSingleTest(prompt, approach) {
  const startTime = Date.now();
  let totalInputTokens = 0;
  let totalOutputTokens = 0;
  let messageCount = 0;

  try {
    const stream = query({
      prompt,
      options: {
        cwd: process.cwd(),
        model: "claude-sonnet-4-6",
        plugins: [
          { type: "local", path: "../" } // Load Kratos plugin
        ]
      }
    });

    for await (const msg of stream) {
      messageCount++;

      if (msg.type === "assistant" && msg.message?.usage) {
        totalInputTokens += msg.message.usage.input_tokens || 0;
        totalOutputTokens += msg.message.usage.output_tokens || 0;
      }

      // Log progress
      if (msg.type === "assistant" && msg.message?.content) {
        process.stdout.write(".");
      }
    }

    console.log(" ✅");

    return {
      inputTokens: totalInputTokens,
      outputTokens: totalOutputTokens,
      duration: Date.now() - startTime,
      messages: messageCount,
      success: true
    };

  } catch (error) {
    console.log(" ❌");
    console.error(`    Error: ${error.message}`);

    return {
      inputTokens: 0,
      outputTokens: 0,
      duration: Date.now() - startTime,
      messages: messageCount,
      success: false,
      error: error.message
    };
  }
}

// Run the comparison if this file is executed directly
if (import.meta.url === `file://${process.argv[1]}`) {
  runComparison().catch(console.error);
}

export { runComparison };