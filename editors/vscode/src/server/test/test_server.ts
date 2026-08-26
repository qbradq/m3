import * as assert from 'assert';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { MarkupContent } from 'vscode-languageserver/node';
import { parseM3Document } from '../parser/m3Parser';
import { parseAsmDocument } from '../parser/asmParser';
import { getM3Completions, getAsmCompletions } from '../providers/completionProvider';
import { getM3Hover, getAsmHover } from '../providers/hoverProvider';

function testM3ParsingAndCompletion() {
  const m3Source = `
package main

// PPU hardware registers
define PPU_CTRL $2000

// Player actor structure
type Player struct {
    x      uint8
    y      uint8
    health uint8
}

// Global player instance in Zero Page
var player Player[4] zp

// Move the player actor
func move_player(dx int8, dy int8) bank 0 {
    player[0].x += dx
}
`;

  const doc = TextDocument.create('file:///test.m3', 'm3', 1, m3Source);
  const parsed = parseM3Document(m3Source);

  // Check symbols parsed
  assert.ok(parsed.symbols.has('PPU_CTRL'), 'PPU_CTRL define should be parsed');
  assert.strictEqual(parsed.symbols.get('PPU_CTRL')?.docComment, 'PPU hardware registers');

  assert.ok(parsed.structs.has('Player'), 'Player struct should be parsed');
  const playerStruct = parsed.structs.get('Player')!;
  assert.strictEqual(playerStruct.fields?.length, 3);
  assert.strictEqual(playerStruct.fields?.[0].name, 'x');

  assert.ok(parsed.symbols.has('player'), 'player variable should be parsed');
  assert.strictEqual(parsed.symbols.get('player')?.storage, 'zp');

  assert.ok(parsed.symbols.has('move_player'), 'move_player function should be parsed');
  assert.strictEqual(parsed.symbols.get('move_player')?.bank, '0');

  // Test general completions
  const completions = getM3Completions(doc, { line: 17, character: 0 }, parsed);
  const completionLabels = completions.map((c) => c.label);

  assert.ok(completionLabels.includes('uint8'), 'Should include uint8 type');
  assert.ok(completionLabels.includes('zp'), 'Should include zp storage');
  assert.ok(completionLabels.includes('func'), 'Should include func keyword');
  assert.ok(completionLabels.includes('low'), 'Should include low intrinsic');
  assert.ok(completionLabels.includes('move_player'), 'Should include user func move_player');
  assert.ok(completionLabels.includes('Player'), 'Should include user struct Player');
  assert.ok(completionLabels.includes('PPU_CTRL'), 'Should include user define PPU_CTRL');

  // Test struct member dot completions: player[0].
  const dotDoc = TextDocument.create('file:///test.m3', 'm3', 2, '    player[0].');
  const memberCompletions = getM3Completions(dotDoc, { line: 0, character: 14 }, parsed);
  const memberLabels = memberCompletions.map((c) => c.label);
  assert.deepStrictEqual(memberLabels, ['x', 'y', 'health'], 'Should suggest struct fields on dot access');
}

function testM3Hover() {
  const m3Source = `
// System frame counter in ZP
var frame_counter uint16 zp

// Initialize the game engine
func init_engine() bank 63 {
    frame_counter = 0
}
`;

  const doc = TextDocument.create('file:///test.m3', 'm3', 1, m3Source);
  const parsed = parseM3Document(m3Source);

  // Hover on user variable
  const varHover = getM3Hover(doc, { line: 2, character: 7 }, parsed);
  assert.ok(varHover, 'Hover on frame_counter should return hover info');
  const varHoverText = (varHover!.contents as MarkupContent).value;
  assert.ok(varHoverText.includes('var frame_counter uint16 zp'));
  assert.ok(varHoverText.includes('System frame counter in ZP'));

  // Hover on built-in type uint16
  const typeHover = getM3Hover(doc, { line: 2, character: 20 }, parsed);
  assert.ok(typeHover, 'Hover on uint16 should return built-in type docs');
  const typeHoverText = (typeHover!.contents as MarkupContent).value;
  assert.ok(typeHoverText.includes('16-bit unsigned integer'));

  // Hover on storage zp
  const storageHover = getM3Hover(doc, { line: 2, character: 25 }, parsed);
  assert.ok(storageHover, 'Hover on zp should return storage documentation');
  const storageHoverText = (storageHover!.contents as MarkupContent).value;
  assert.ok(storageHoverText.includes('Zero Page RAM'));

  // Hover on user function
  const funcHover = getM3Hover(doc, { line: 5, character: 7 }, parsed);
  assert.ok(funcHover, 'Hover on init_engine should return function signature & comments');
  const funcHoverText = (funcHover!.contents as MarkupContent).value;
  assert.ok(funcHoverText.includes('func init_engine()'));
  assert.ok(funcHoverText.includes('Initialize the game engine'));
}

function testAsmParsingAndCompletion() {
  const asmSource = `
.bank 0

; Main reset vector entry point
.proc reset_handler
    SEI
    CLD
    LDX #$FF
    TXS
.endproc

; Quick helper macro for setting PPU address
.macro set_ppu_addr addr
    LDA #>addr
    STA $2006
    LDA #<addr
    STA $2006
.endmacro

player_score: .res 2
`;

  const doc = TextDocument.create('file:///test.s', 'm3-asm', 1, asmSource);
  const parsed = parseAsmDocument(asmSource);

  assert.ok(parsed.symbols.has('reset_handler'), 'Should parse reset_handler proc');
  assert.strictEqual(parsed.symbols.get('reset_handler')?.bank, '0');
  assert.strictEqual(parsed.symbols.get('reset_handler')?.docComment, 'Main reset vector entry point');

  assert.ok(parsed.macros.has('set_ppu_addr'), 'Should parse set_ppu_addr macro');
  assert.strictEqual(parsed.macros.get('set_ppu_addr')?.args?.[0], 'addr');

  assert.ok(parsed.symbols.has('player_score'), 'Should parse player_score symbol');

  // Test general assembly completions
  const completions = getAsmCompletions(doc, { line: 17, character: 0 }, parsed);
  const labels = completions.map((c) => c.label);

  assert.ok(labels.includes('LDA'), 'Should include LDA instruction');
  assert.ok(labels.includes('STA'), 'Should include STA instruction');
  assert.ok(labels.includes('JSR'), 'Should include JSR instruction');
  assert.ok(labels.includes('.bank'), 'Should include .bank directive');
  assert.ok(labels.includes('.proc'), 'Should include .proc directive');
  assert.ok(labels.includes('PPU_CTRL'), 'Should include PPU_CTRL hardware register');
  assert.ok(labels.includes('reset_handler'), 'Should include user proc reset_handler');
  assert.ok(labels.includes('set_ppu_addr'), 'Should include user macro set_ppu_addr');

  // Test directive completion when typing dot '.'
  const dotDoc = TextDocument.create('file:///test.s', 'm3-asm', 2, '    .');
  const dotCompletions = getAsmCompletions(dotDoc, { line: 0, character: 5 }, parsed);
  const dotLabels = dotCompletions.map((c) => c.label);
  assert.ok(dotLabels.includes('.incchr'), 'Should include .incchr on dot trigger');
  assert.ok(dotLabels.includes('.asciiz'), 'Should include .asciiz on dot trigger');
}

function testAsmHover() {
  const asmSource = `
; Sound initialization routine
.proc init_audio
    LDA #$00
    STA $4015
    RTS
.endproc
`;

  const doc = TextDocument.create('file:///test.s', 'm3-asm', 1, asmSource);
  const parsed = parseAsmDocument(asmSource);

  // Hover on 6502 instruction LDA
  const ldaHover = getAsmHover(doc, { line: 3, character: 6 }, parsed);
  assert.ok(ldaHover, 'Hover on LDA should return instruction documentation');
  const ldaHoverText = (ldaHover!.contents as MarkupContent).value;
  assert.ok(ldaHoverText.includes('Load Accumulator'));
  assert.ok(ldaHoverText.includes('N Z'));

  // Hover on directive .proc
  const procHover = getAsmHover(doc, { line: 2, character: 2 }, parsed);
  assert.ok(procHover, 'Hover on .proc should return directive documentation');
  const procHoverText = (procHover!.contents as MarkupContent).value;
  assert.ok(procHoverText.includes('.proc'));

  // Hover on user procedure
  const userProcHover = getAsmHover(doc, { line: 2, character: 8 }, parsed);
  assert.ok(userProcHover, 'Hover on init_audio should return proc info and doc comments');
  const userProcHoverText = (userProcHover!.contents as MarkupContent).value;
  assert.ok(userProcHoverText.includes('.proc init_audio'));
  assert.ok(userProcHoverText.includes('Sound initialization routine'));

  // Hover on NES register PPU_STATUS
  const regDoc = TextDocument.create('file:///test.s', 'm3-asm', 2, '    BIT PPU_STATUS');
  const regHover = getAsmHover(regDoc, { line: 0, character: 10 }, parsed);
  assert.ok(regHover, 'Hover on PPU_STATUS should return register info');
  const regHoverText = (regHover!.contents as MarkupContent).value;
  assert.ok(regHoverText.includes('PPU_STATUS ($2002)'));
  assert.ok(regHoverText.includes('VBlank flag'));
}

function main() {
  console.log('Running Language Server Unit Tests...');
  testM3ParsingAndCompletion();
  console.log('✓ m3 Parsing & Symbol Completion Passed');
  testM3Hover();
  console.log('✓ m3 Symbol Hover & Documentation Passed');
  testAsmParsingAndCompletion();
  console.log('✓ m3-asm Parsing & Completion Passed');
  testAsmHover();
  console.log('✓ m3-asm Symbol Hover & Documentation Passed');
  console.log('\nAll Language Server Tests Passed Successfully!');
}

main();
