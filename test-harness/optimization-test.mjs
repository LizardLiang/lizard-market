#!/usr/bin/env node

/**
 * Direct Optimization Comparison
 * Runs same task with context optimization vs original approach
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import { writeFileSync } from "fs";

console.log("🧪 Direct Context Optimization Test");
console.log("===================================");

// Baseline from previous test
const baseline = {
  approach: "original",
  inputTokens: 34690,
  outputTokens: 3056,
  totalTokens: 37746,
  duration: 129.2,
  messages: 36
};

console.log("📊 Baseline (Original Approach):");
console.log(`   Input: ${baseline.inputTokens.toLocaleString()} tokens`);
console.log(`   Output: ${baseline.outputTokens.toLocaleString()} tokens`);
console.log(`   Total: ${baseline.totalTokens.toLocaleString()} tokens`);
console.log(`   Duration: ${baseline.duration}s`);

// Test the context-optimized approach
console.log("\n🟢 Testing optimized approach...");

const optimizedPrompt = `
CURATED_CONTEXT:
{
  "role": "Planning Agent",
  "project_overview": "Claude Code plugin development project with Kratos orchestration system",
  "tech_stack": "Node.js, Claude Agent SDK, markdown-based agent definitions",
  "planning_template": "Project breakdown with task identification and implementation steps",
  "requirements": "CLI utility in Go for JSON config file processing and formatted output"
}

Task: Plan implementation for a CLI utility in Go that reads a JSON config file and prints a formatted summary table to stdout. It should support --config flag and --output flag for plain or json formats.

You have all necessary context above. Create an implementation plan without reading additional files.
`;

async function testOptimized() {
  const startTime = Date.now();
  let inputTokens = 0;
  let outputTokens = 0;
  let messages = 0;

  try {
    const stream = query({
      prompt: optimizedPrompt,
      options: {
        model: "claude-sonnet-4-6",
        cwd: process.cwd()
      }
    });

    for await (const msg of stream) {
      messages++;

      if (msg.type === "assistant" && msg.message?.usage) {
        inputTokens += msg.message.usage.input_tokens || 0;
        outputTokens += msg.message.usage.output_tokens || 0;
      }

      if (messages % 5 === 0) process.stdout.write(".");
    }

    console.log(" ✅");

    const optimized = {
      approach: "context-optimized",
      inputTokens,
      outputTokens,
      totalTokens: inputTokens + outputTokens,
      duration: (Date.now() - startTime) / 1000,
      messages
    };

    console.log("\n📊 Optimized Results:");
    console.log(`   Input: ${optimized.inputTokens.toLocaleString()} tokens`);
    console.log(`   Output: ${optimized.outputTokens.toLocaleString()} tokens`);
    console.log(`   Total: ${optimized.totalTokens.toLocaleString()} tokens`);
    console.log(`   Duration: ${optimized.duration}s`);

    // Calculate savings
    const inputSavings = baseline.inputTokens - optimized.inputTokens;
    const inputSavingsPercent = (inputSavings / baseline.inputTokens * 100).toFixed(1);
    const totalSavings = baseline.totalTokens - optimized.totalTokens;
    const totalSavingsPercent = (totalSavings / baseline.totalTokens * 100).toFixed(1);

    console.log("\n🎯 COMPARISON RESULTS:");
    console.log(`   Input token savings: ${inputSavings.toLocaleString()} (${inputSavingsPercent}%)`);
    console.log(`   Total token savings: ${totalSavings.toLocaleString()} (${totalSavingsPercent}%)`);
    console.log(`   Speed improvement: ${((baseline.duration - optimized.duration) / baseline.duration * 100).toFixed(1)}%`);

    // Final verdict
    if (parseFloat(inputSavingsPercent) >= 60) {
      console.log("\n✅ OPTIMIZATION SUCCESS!");
      console.log(`   Target: 60-75% input reduction`);
      console.log(`   Achieved: ${inputSavingsPercent}%`);
    } else {
      console.log("\n❌ OPTIMIZATION INSUFFICIENT");
      console.log(`   Target: 60-75% input reduction`);
      console.log(`   Achieved: ${inputSavingsPercent}%`);
    }

    // Save results
    const results = {
      timestamp: new Date().toISOString(),
      baseline,
      optimized,
      savings: {
        inputTokens: inputSavings,
        inputPercent: inputSavingsPercent,
        totalTokens: totalSavings,
        totalPercent: totalSavingsPercent,
        speedPercent: ((baseline.duration - optimized.duration) / baseline.duration * 100).toFixed(1)
      },
      verdict: parseFloat(inputSavingsPercent) >= 60 ? "SUCCESS" : "INSUFFICIENT"
    };

    writeFileSync("optimization-test-results.json", JSON.stringify(results, null, 2));
    console.log(`\nResults saved to: optimization-test-results.json`);

    return results;

  } catch (error) {
    console.log(` ❌ Error: ${error.message}`);
    return null;
  }
}

testOptimized();