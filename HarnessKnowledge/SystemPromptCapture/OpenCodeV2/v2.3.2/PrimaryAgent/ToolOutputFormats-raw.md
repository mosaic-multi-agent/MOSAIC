# Tool Output Formats — Raw Captures

| Field | Value |
|-------|-------|
| **Harness** | OpenCodeV2 |
| **Harness version** | v2.3.2 |
| **Context** | Primary agent session |
| **Captured** | 2026-11-18 |
| **Formatting** | XML tags use bracket notation: `<tag>` -> `[tag]` to avoid rendering conflicts |

---

## functions.read — full small file

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt"})`

--- RAW OUTPUT START ---
Read file HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt, lines 1-20
1: Line 1: Greetings, Universe! 🚀
2: Line 2: This file has "double quotes" and 'single quotes' for validation.
3: Line 3:     Four-space indentation here.
4: Line 4:	Tab-indented content here.
5: Line 5: Special characters: [chevrons] & ampersand | pipe \ backslash
6: Line 6: Unicode: resume, naive, 中文测试, Кириллица, עברית
7: Line 7: Blank line follows:
8: 
9: Line 9: Resumed after blank line.
10: Line 10: Path-like content: /opt/tools/bin/run --mode=capture
11: Line 11: Code-like content: const calc = (x) => { return x > 0 && x < 100; }
12: Line 12: Regex-like content: ^begin.*center[a-z]+finish$
13: Line 13: XML-like content: [node role="test"]nested value[/node]
14: Line 14: JSON-like content: {"name": "probe", "count": 77, "items": [4, 5, 6]}
15: --- EDIT TARGET START ---
16: Line 16: Swap this line with different content.
17: --- EDIT TARGET END ---
18: Line 18: This line must stay intact after edit tests.
19: Line 19: Grep target: DISTINCT_MATCH_TOKEN_ABC
20: Line 20: Final line with trailing newline.
--- RAW OUTPUT END ---

## functions.read — partial file

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt","offset":3,"limit":4})`

--- RAW OUTPUT START ---
Read file HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt, lines 3-6
3: Line 3:     Four-space indentation here.
4:	Tab-indented content here.
5: Line 5: Special characters: [chevrons] & ampersand | pipe \ backslash
6: Line 6: Unicode: resume, naive, 中文测试, Кириллица, עברית
[Output truncated. Continue reading with offset: 7]
--- RAW OUTPUT END ---

## functions.read — directory

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
Read directory HarnessKnowledge/SystemPromptCapture/TestArtifacts, entries 1-2
large-file-3000-lines.txt
varied-content-test.txt
--- RAW OUTPUT END ---

## functions.read — file not found

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/nonexistent.txt"})`

--- RAW OUTPUT START ---
{"error":{"type":"tool.execution","message":"File not found: HarnessKnowledge/SystemPromptCapture/TestArtifacts/nonexistent.txt"},"content":[]}
--- RAW OUTPUT END ---

## functions.read — large file default limit

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/large-file-3000-lines.txt"})`

--- RAW OUTPUT START ---
Read file HarnessKnowledge/SystemPromptCapture/TestArtifacts/large-file-3000-lines.txt, lines 1-1422
1: foxtrot golf [ROW:0001] hotel india
2: foxtrot golf [ROW:0002] hotel india
3: foxtrot golf [ROW:0003] hotel india
4: foxtrot golf [ROW:0004] hotel india
5: foxtrot golf [ROW:0005] hotel india
6: foxtrot golf [ROW:0006] hotel india
7: foxtrot golf [ROW:0007] hotel india
8: foxtrot golf [ROW:0008] hotel india
9: foxtrot golf [ROW:0009] hotel india
10: foxtrot golf [ROW:0010] hotel india
11: foxtrot golf [ROW:0011] hotel india
12: foxtrot golf [ROW:0012] hotel india
13: foxtrot golf [ROW:0013] hotel india
14: foxtrot golf [ROW:0014] hotel india
15: foxtrot golf [ROW:0015] hotel india
16: foxtrot golf [ROW:0016] hotel india
17: foxtrot golf [ROW:0017] hotel india
18: foxtrot golf [ROW:0018] hotel india
19: foxtrot golf [ROW:0019] hotel india
20: foxtrot golf [ROW:0020] hotel india
21: foxtrot golf [ROW:0021] hotel india
22: foxtrot golf [ROW:0022] hotel india
23: foxtrot golf [ROW:0023] hotel india
24: foxtrot golf [ROW:0024] hotel india
25: foxtrot golf [ROW:0025] hotel india
26: foxtrot golf [ROW:0026] hotel india
27: foxtrot golf [ROW:0027] hotel india
28: foxtrot golf [ROW:0028] hotel india
29: foxtrot golf [ROW:0029] hotel india
30: foxtrot golf [ROW:0030] hotel india
31: foxtrot golf [ROW:0031] hotel india
32: foxtrot golf [ROW:0032] hotel india
33: foxtrot golf [ROW:0033] hotel india
34: foxtrot golf [ROW:0034] hotel india
35: foxtrot golf [ROW:0035] hotel india
36: foxtrot golf [ROW:0036] hotel india
37: foxtrot golf [ROW:0037] hotel india
38: foxtrot golf [ROW:0038] hotel india
39: foxtrot golf [ROW:0039] hotel india
40: foxtrot golf [ROW:0040] hotel india
41: foxtrot golf [ROW:0041] hotel india
42: foxtrot golf [ROW:0042] hotel india
43: foxtrot golf [ROW:0043] hotel india
44: foxtrot golf [ROW:0044] hotel india
45: foxtrot golf [ROW:0045] hotel india
46: foxtrot golf [ROW:0046] hotel india
47: foxtrot golf [ROW:0047] hotel india
48: foxtrot golf [ROW:0048] hotel india
49: foxtrot golf [ROW:0049] hotel india
50: foxtrot golf [ROW:0050] hotel india
51: foxtrot golf [ROW:0051] hotel india
52: foxtrot golf [ROW:0052] hotel india
53: foxtrot golf [ROW:0053] hotel india
54: foxtrot golf [ROW:0054] hotel india
55: foxtrot golf [ROW:0055] hotel india
56: foxtrot golf [ROW:0056] hotel india
57: foxtrot golf [ROW:0057] hotel india
58: foxtrot golf [ROW:0058] hotel india
59: foxtrot golf [ROW:0059] hotel india
60: foxtrot golf [ROW:0060] hotel india
61: foxtrot golf [ROW:0061] hotel india
62: foxtrot golf [ROW:0062] hotel india
63: foxtrot golf [ROW:0063] hotel india
64: foxtrot golf [ROW:0064] hotel india
65: foxtrot golf [ROW:0065] hotel india
66: foxtrot golf [ROW:0066] hotel india
67: foxtrot golf [ROW:0067] hotel india
68: foxtrot golf [ROW:0068] hotel india
69: foxtrot golf [ROW:0069] hotel india
70: foxtrot golf [ROW:0070] hotel india
71: foxtrot golf [ROW:0071] hotel india
72: foxtrot golf [ROW:0072] hotel india
73: foxtrot golf [ROW:0073] hotel india
74: foxtrot golf [ROW:0074] hotel india
75: foxtrot golf [ROW:0075] hotel india
76: foxtrot golf [ROW:0076] hotel india
77: foxtrot golf [ROW:0077] hotel india
78: foxtrot golf [ROW:0078] hotel india
79: foxtrot golf [ROW:0079] hotel india
80: foxtrot golf [ROW:0080] hotel india
81: foxtrot golf [ROW:0081] hotel india
82: foxtrot golf [ROW:0082] hotel india
83: foxtrot golf [ROW:0083] hotel india
84: foxtrot golf [ROW:0084] hotel india
85: foxtrot golf [ROW:0085] hotel india
86: foxtrot golf [ROW:0086] hotel india
87: foxtrot golf [ROW:0087] hotel india
88: foxtrot golf [ROW:0088] hotel india
89: foxtrot golf [ROW:0089] hotel india
90: foxtrot golf [ROW:0090] hotel india
91: foxtrot golf [ROW:0091] hotel india
92: foxtrot golf [ROW:0092] hotel india
93: foxtrot golf [ROW:0093] hotel india
94: foxtrot golf [ROW:0094] hotel india
95: foxtrot golf [ROW:0095] hotel india
96: foxtrot golf [ROW:0096] hotel india
97: foxtrot golf [ROW:0097] hotel india
98: foxtrot golf [ROW:0098] hotel india
99: foxtrot golf [ROW:0099] hotel india
100: foxtrot golf [ROW:0100] hotel india
101: foxtrot golf [ROW:0101] hotel india
102: foxtrot golf [ROW:0102] hotel india
103: foxtrot golf [ROW:0103] hotel india
104: foxtrot golf [ROW:0104] hotel india
105: foxtrot golf [ROW:0105] hotel india
106: foxtrot golf [ROW:0106] hotel india
107: foxtrot golf [ROW:0107] hotel india
108: foxtrot golf [ROW:0108] hotel india
109: foxtrot golf [ROW:0109] hotel india
110: foxtrot golf [ROW:0110] hotel india
111: foxtrot golf [ROW:0111] hotel india
112: foxtrot golf [ROW:0112] hotel india
113: foxtrot golf [ROW:0113] hotel india
114: foxtrot golf [ROW:0114] hotel india
115: foxtrot golf [ROW:0115] hotel india
116: foxtrot golf [ROW:0116] hotel india
117: foxtrot golf [ROW:0117] hotel india
118: foxtrot golf [ROW:0118] hotel india
119: foxtrot golf [ROW:0119] hotel india
120: foxtrot golf [ROW:0120] hotel india
121: foxtrot golf [ROW:0121] hotel india
122: foxtrot golf [ROW:0122] hotel india
123: foxtrot golf [ROW:0123] hotel india
124: foxtrot golf [ROW:0124] hotel india
125: foxtrot golf [ROW:0125] hotel india
126: foxtrot golf [ROW:0126] hotel india
127: foxtrot golf [ROW:0127] hotel india
128: foxtrot golf [ROW:0128] hotel india
129: foxtrot golf [ROW:0129] hotel india
130: foxtrot golf [ROW:0130] hotel india
131: foxtrot golf [ROW:0131] hotel india
132: foxtrot golf [ROW:0132] hotel india
133: foxtrot golf [ROW:0133] hotel india
134: foxtrot golf [ROW:0134] hotel india
135: foxtrot golf [ROW:0135] hotel india
136: foxtrot golf [ROW:0136] hotel india
137: foxtrot golf [ROW:0137] hotel india
138: foxtrot golf [ROW:0138] hotel india
139: foxtrot golf [ROW:0139] hotel india
140: foxtrot golf [ROW:0140] hotel india
141: foxtrot golf [ROW:0141] hotel india
142: foxtrot golf [ROW:0142] hotel india
143: foxtrot golf [ROW:0143] hotel india
144: foxtrot golf [ROW:0144] hotel india
145: foxtrot golf [ROW:0145] hotel india
146: foxtrot golf [ROW:0146] hotel india
147: foxtrot golf [ROW:0147] hotel india
148: foxtrot golf [ROW:0148] hotel india
149: foxtrot golf [ROW:0149] hotel india
150: foxtrot golf [ROW:0150] hotel india
151: foxtrot golf [ROW:0151] hotel india
152: foxtrot golf [ROW:0152] hotel india
153: foxtrot golf [ROW:0153] hotel india
154: foxtrot golf [ROW:0154] hotel india
155: foxtrot golf [ROW:0155] hotel india
156: foxtrot golf [ROW:0156] hotel india
157: foxtrot golf [ROW:0157] hotel india
158: foxtrot golf [ROW:0158] hotel india
159: foxtrot golf [ROW:0159] hotel india
160: foxtrot golf [ROW:0160] hotel india
161: foxtrot golf [ROW:0161] hotel india
162: foxtrot golf [ROW:0162] hotel india
163: foxtrot golf [ROW:0163] hotel india
164: foxtrot golf [ROW:0164] hotel india
165: foxtrot golf [ROW:0165] hotel india
166: foxtrot golf [ROW:0166] hotel india
167: foxtrot golf [ROW:0167] hotel india
168: foxtrot golf [ROW:0168] hotel india
169: foxtrot golf [ROW:0169] hotel india
170: foxtrot golf [ROW:0170] hotel india
171: foxtrot golf [ROW:0171] hotel india
172: foxtrot golf [ROW:0172] hotel india
173: foxtrot golf [ROW:0173] hotel india
174: foxtrot golf [ROW:0174] hotel india
175: foxtrot golf [ROW:0175] hotel india
176: foxtrot golf [ROW:0176] hotel india
177: foxtrot golf [ROW:0177] hotel india
178: foxtrot golf [ROW:0178] hotel india
179: foxtrot golf [ROW:0179] hotel india
180: foxtrot golf [ROW:0180] hotel india
181: foxtrot golf [ROW:0181] hotel india
182: foxtrot golf [ROW:0182] hotel india
183: foxtrot golf [ROW:0183] hotel india
184: foxtrot golf [ROW:0184] hotel india
185: foxtrot golf [ROW:0185] hotel india
186: foxtrot golf [ROW:0186] hotel india
187: foxtrot golf [ROW:0187] hotel india
188: foxtrot golf [ROW:0188] hotel india
189: foxtrot golf [ROW:0189] hotel india
190: foxtrot golf [ROW:0190] hotel india
191: foxtrot golf [ROW:0191] hotel india
192: foxtrot golf [ROW:0192] hotel india
193: foxtrot golf [ROW:0193] hotel india
194: foxtrot golf [ROW:0194] hotel india
195: foxtrot golf [ROW:0195] hotel india
196: foxtrot golf [ROW:0196] hotel india
197: foxtrot golf [ROW:0197] hotel india
198: foxtrot golf [ROW:0198] hotel india
199: foxtrot golf [ROW:0199] hotel india
200: foxtrot golf [ROW:0200] hotel india
201: foxtrot golf [ROW:0201] hotel india
202: foxtrot golf [ROW:0202] hotel india
203: foxtrot golf [ROW:0203] hotel india
204: foxtrot golf [ROW:0204] hotel india
205: foxtrot golf [ROW:0205] hotel india
206: foxtrot golf [ROW:0206] hotel india
207: foxtrot golf [ROW:0207] hotel india
208: foxtrot golf [ROW:0208] hotel india
209: foxtrot golf [ROW:0209] hotel india
210: foxtrot golf [ROW:0210] hotel india
211: foxtrot golf [ROW:0211] hotel india
212: foxtrot golf [ROW:0212] hotel india
213: foxtrot golf [ROW:0213] hotel india
214: foxtrot golf [ROW:0214] hotel india
215: foxtrot golf [ROW:0215] hotel india
216: foxtrot golf [ROW:0216] hotel india
217: foxtrot golf [ROW:0217] hotel india
218: foxtrot golf [ROW:0218] hotel india
219: foxtrot golf [ROW:0219] hotel india
220: foxtrot golf [ROW:0220] hotel india
221: foxtrot golf [ROW:0221] hotel india
222: foxtrot golf [ROW:0222] hotel india
223: foxtrot golf [ROW:0223] hotel india
224: foxtrot golf [ROW:0224] hotel india
225: foxtrot golf [ROW:0225] hotel india
226: foxtrot golf [ROW:0226] hotel india
227: foxtrot golf [ROW:0227] hotel india
228: foxtrot golf [ROW:0228] hotel india
229: foxtrot golf [ROW:0229] hotel india
230: foxtrot golf [ROW:0230] hotel india
231: foxtrot golf [ROW:0231] hotel india
232: foxtrot golf [ROW:0232] hotel india
233: foxtrot golf [ROW:0233] hotel india
234: foxtrot golf [ROW:0234] hotel india
235: foxtrot golf [ROW:0235] hotel india
236: foxtrot golf [ROW:0236] hotel india
237: foxtrot golf [ROW:0237] hotel india
238: foxtrot golf [ROW:0238] hotel india
239: foxtrot golf [ROW:0239] hotel india
240: foxtrot golf [ROW:0240] hotel india
241: foxtrot golf [ROW:0241] hotel india
242: foxtrot golf [ROW:0242] hotel india
243: foxtrot golf [ROW:0243] hotel india
244: foxtrot golf [ROW:0244] hotel india
245: foxtrot golf [ROW:0245] hotel india
246: foxtrot golf [ROW:0246] hotel india
247: foxtrot golf [ROW:0247] hotel india
248: foxtrot golf [ROW:0248] hotel india
249: foxtrot golf [ROW:0249] hotel india
250: foxtrot golf [ROW:0250] hotel india
251: foxtrot golf [ROW:0251] hotel india
252: foxtrot golf [ROW:0252] hotel india
253: foxtrot golf [ROW:0253] hotel india
254: foxtrot golf [ROW:0254] hotel india
255: foxtrot golf [ROW:0255] hotel india
256: foxtrot golf [ROW:0256] hotel india
257: foxtrot golf [ROW:0257] hotel india
258: foxtrot golf [ROW:0258] hotel india
259: foxtrot golf [ROW:0259] hotel india
260: foxtrot golf [ROW:0260] hotel india
261: foxtrot golf [ROW:0261] hotel india
262: foxtrot golf [ROW:0262] hotel india
263: foxtrot golf [ROW:0263] hotel india
264: foxtrot golf [ROW:0264] hotel india
265: foxtrot golf [ROW:0265] hotel india
266: foxtrot golf [ROW:0266] hotel india
267: foxtrot golf [ROW:0267] hotel india
268: foxtrot golf [ROW:0268] hotel india
269: foxtrot golf [ROW:0269] hotel india
270: foxtrot golf [ROW:0270] hotel india
271: foxtrot golf [ROW:0271] hotel india
272: foxtrot golf [ROW:0272] hotel india
273: foxtrot golf [ROW:0273] hotel india
274: foxtrot golf [ROW:0274] hotel india
275: foxtrot golf [ROW:0275] hotel india
276: foxtrot golf [ROW:0276] hotel india
277: foxtrot golf [ROW:0277] hotel india
278: foxtrot golf [ROW:0278] hotel india
279: foxtrot golf [ROW:0279] hotel india
280: foxtrot golf [ROW:0280] hotel india
281: foxtrot golf [ROW:0281] hotel india
282: foxtrot golf [ROW:0282] hotel india
283: foxtrot golf [ROW:0283] hotel india
284: foxtrot golf [ROW:0284] hotel india
285: foxtrot golf [ROW:0285] hotel india
286: foxtrot golf [ROW:0286] hotel india
287: foxtrot golf [ROW:0287] hotel india
288: foxtrot golf [ROW:0288] hotel india
289: foxtrot golf [ROW:0289] hotel india
290: foxtrot golf [ROW:0290] hotel india
291: foxtrot golf [ROW:0291] hotel india
292: foxtrot golf [ROW:0292] hotel india
293: foxtrot golf [ROW:0293] hotel india
294: foxtrot golf [ROW:0294] hotel india
295: foxtrot golf [ROW:0295] hotel india
296: foxtrot golf [ROW:0296] hotel india
297: foxtrot golf [ROW:0297] hotel india
298: foxtrot golf [ROW:0298] hotel india
299: foxtrot golf [ROW:0299] hotel india
300: foxtrot golf [ROW:0300] hotel india
301: foxtrot golf [ROW:0301] hotel india
302: foxtrot golf [ROW:0302] hotel india
303: foxtrot golf [ROW:0303] hotel india
304: foxtrot golf [ROW:0304] hotel india
305: foxtrot golf [ROW:0305] hotel india
306: foxtrot golf [ROW:0306] hotel india
307: foxtrot golf [ROW:0307] hotel india
308: foxtrot golf [ROW:0308] hotel india
309: foxtrot golf [ROW:0309] hotel india
310: foxtrot golf [ROW:0310] hotel india
311: foxtrot golf [ROW:0311] hotel india
312: foxtrot golf [ROW:0312] hotel india
313: foxtrot golf [ROW:0313] hotel india
314: foxtrot golf [ROW:0314] hotel india
315: foxtrot golf [ROW:0315] hotel india
316: foxtrot golf [ROW:0316] hotel india
317: foxtrot golf [ROW:0317] hotel india
318: foxtrot golf [ROW:0318] hotel india
319: foxtrot golf [ROW:0319] hotel india
320: foxtrot golf [ROW:0320] hotel india
321: foxtrot golf [ROW:0321] hotel india
322: foxtrot golf [ROW:0322] hotel india
323: foxtrot golf [ROW:0323] hotel india
324: foxtrot golf [ROW:0324] hotel india
325: foxtrot golf [ROW:0325] hotel india
326: foxtrot golf [ROW:0326] hotel india
327: foxtrot golf [ROW:0327] hotel india
328: foxtrot golf [ROW:0328] hotel india
329: foxtrot golf [ROW:0329] hotel india
330: foxtrot golf [ROW:0330] hotel india
331: foxtrot golf [ROW:0331] hotel india
332: foxtrot golf [ROW:0332] hotel india
333: foxtrot golf [ROW:0333] hotel india
334: foxtrot golf [ROW:0334] hotel india
335: foxtrot golf [ROW:0335] hotel india
336: foxtrot golf [ROW:0336] hotel india
337: foxtrot golf [ROW:0337] hotel india
338: foxtrot golf [ROW:0338] hotel india
339: foxtrot golf [ROW:0339] hotel india
340: foxtrot golf [ROW:0340] hotel india
341: foxtrot golf [ROW:0341] hotel india
342: foxtrot golf [ROW:0342] hotel india
343: foxtrot golf [ROW:0343] hotel india
344: foxtrot golf [ROW:0344] hotel india
345: foxtrot golf [ROW:0345] hotel india
346: foxtrot golf [ROW:0346] hotel india
347: foxtrot golf [ROW:0347] hotel india
348: foxtrot golf [ROW:0348] hotel india
349: foxtrot golf [ROW:0349] hotel india
350: foxtrot golf [ROW:0350] hotel india
351: foxtrot golf [ROW:0351] hotel india
352: foxtrot golf [ROW:0352] hotel india
353: foxtrot golf [ROW:0353] hotel india
354: foxtrot golf [ROW:0354] hotel india
355: foxtrot golf [ROW:0355] hotel india
356: foxtrot golf [ROW:0356] hotel india
357: foxtrot golf [ROW:0357] hotel india
358: foxtrot golf [ROW:0358] hotel india
359: foxtrot golf [ROW:0359] hotel india
360: foxtrot golf [ROW:0360] hotel india
361: foxtrot golf [ROW:0361] hotel india
362: foxtrot golf [ROW:0362] hotel india
363: foxtrot golf [ROW:0363] hotel india
364: foxtrot golf [ROW:0364] hotel india
365: foxtrot golf [ROW:0365] hotel india
366: foxtrot golf [ROW:0366] hotel india
367: foxtrot golf [ROW:0367] hotel india
368: foxtrot golf [ROW:0368] hotel india
369: foxtrot golf [ROW:0369] hotel india
370: foxtrot golf [ROW:0370] hotel india
371: foxtrot golf [ROW:0371] hotel india
372: foxtrot golf [ROW:0372] hotel india
373: foxtrot golf [ROW:0373] hotel india
374: foxtrot golf [ROW:0374] hotel india
375: foxtrot golf [ROW:0375] hotel india
376: foxtrot golf [ROW:0376] hotel india
377: foxtrot golf [ROW:0377] hotel india
378: foxtrot golf [ROW:0378] hotel india
379: foxtrot golf [ROW:0379] hotel india
380: foxtrot golf [ROW:0380] hotel india
381: foxtrot golf [ROW:0381] hotel india
382: foxtrot golf [ROW:0382] hotel india
383: foxtrot golf [ROW:0383] hotel india
384: foxtrot golf [ROW:0384] hotel india
385: foxtrot golf [ROW:0385] hotel india
386: foxtrot golf [ROW:0386] hotel india
387: foxtrot golf [ROW:0387] hotel india
388: foxtrot golf [ROW:0388] hotel india
389: foxtrot golf [ROW:0389] hotel india
390: foxtrot golf [ROW:0390] hotel india
391: foxtrot golf [ROW:0391] hotel india
392: foxtrot golf [ROW:0392] hotel india
393: foxtrot golf [ROW:0393] hotel india
394: foxtrot golf [ROW:0394] hotel india
395: foxtrot golf [ROW:0395] hotel india
396: foxtrot golf [ROW:0396] hotel india
397: foxtrot golf [ROW:0397] hotel india
398: foxtrot golf [ROW:0398] hotel india
399: foxtrot golf [ROW:0399] hotel india
400: foxtrot golf [ROW:0400] hotel india
401: foxtrot golf [ROW:0401] hotel india
402: foxtrot golf [ROW:0402] hotel india
403: foxtrot golf [ROW:0403] hotel india
404: foxtrot golf [ROW:0404] hotel india
405: foxtrot golf [ROW:0405] hotel india
406: foxtrot golf [ROW:0406] hotel india
407: foxtrot golf [ROW:0407] hotel india
408: foxtrot golf [ROW:0408] hotel india
409: foxtrot golf [ROW:0409] hotel india
410: foxtrot golf [ROW:0410] hotel india
411: foxtrot golf [ROW:0411] hotel india
412: foxtrot golf [ROW:0412] hotel india
413: foxtrot golf [ROW:0413] hotel india
414: foxtrot golf [ROW:0414] hotel india
415: foxtrot golf [ROW:0415] hotel india
416: foxtrot golf [ROW:0416] hotel india
417: foxtrot golf [ROW:0417] hotel india
418: foxtrot golf [ROW:0418] hotel india
419: foxtrot golf [ROW:0419] hotel india
420: foxtrot golf [ROW:0420] hotel india
421: foxtrot golf [ROW:0421] hotel india
422: foxtrot golf [ROW:0422] hotel india
423: foxtrot golf [ROW:0423] hotel india
424: foxtrot golf [ROW:0424] hotel india
425: foxtrot golf [ROW:0425] hotel india
426: foxtrot golf [ROW:0426] hotel india
427: foxtrot golf [ROW:0427] hotel india
428: foxtrot golf [ROW:0428] hotel india
429: foxtrot golf [ROW:0429] hotel india
430: foxtrot golf [ROW:0430] hotel india
431: foxtrot golf [ROW:0431] hotel india
432: foxtrot golf [ROW:0432] hotel india
433: foxtrot golf [ROW:0433] hotel india
434: foxtrot golf [ROW:0434] hotel india
435: foxtrot golf [ROW:0435] hotel india
436: foxtrot golf [ROW:0436] hotel india
437: foxtrot golf [ROW:0437] hotel india
438: foxtrot golf [ROW:0438] hotel india
439: foxtrot golf [ROW:0439] hotel india
440: foxtrot golf [ROW:0440] hotel india
441: foxtrot golf [ROW:0441] hotel india
442: foxtrot golf [ROW:0442] hotel india
443: foxtrot golf [ROW:0443] hotel india
444: foxtrot golf [ROW:0444] hotel india
445: foxtrot golf [ROW:0445] hotel india
446: foxtrot golf [ROW:0446] hotel india
447: foxtrot golf [ROW:0447] hotel india
448: foxtrot golf [ROW:0448] hotel india
449: foxtrot golf [ROW:0449] hotel india
450: foxtrot golf [ROW:0450] hotel india
451: foxtrot golf [ROW:0451] hotel india
452: foxtrot golf [ROW:0452] hotel india
453: foxtrot golf [ROW:0453] hotel india
454: foxtrot golf [ROW:0454] hotel india
455: foxtrot golf [ROW:0455] hotel india
456: foxtrot golf [ROW:0456] hotel india
457: foxtrot golf [ROW:0457] hotel india
458: foxtrot golf [ROW:0458] hotel india
459: foxtrot golf [ROW:0459] hotel india
460: foxtrot golf [ROW:0460] hotel india
461: foxtrot golf [ROW:0461] hotel india
462: foxtrot golf [ROW:0462] hotel india
463: foxtrot golf [ROW:0463] hotel india
464: foxtrot golf [ROW:0464] hotel india
465: foxtrot golf [ROW:0465] hotel india
466: foxtrot golf [ROW:0466] hotel india
467: foxtrot golf [ROW:0467] hotel india
468: foxtrot golf [ROW:0468] hotel india
469: foxtrot golf [ROW:0469] hotel india
470: foxtrot golf [ROW:0470] hotel india
471: foxtrot golf [ROW:0471] hotel india
472: foxtrot golf [ROW:0472] hotel india
473: foxtrot golf [ROW:0473] hotel india
474: foxtrot golf [ROW:0474] hotel india
475: foxtrot golf [ROW:0475] hotel india
476: foxtrot golf [ROW:0476] hotel india
477: foxtrot golf [ROW:0477] hotel india
478: foxtrot golf [ROW:0478] hotel india
479: foxtrot golf [ROW:0479] hotel india
480: foxtrot golf [ROW:0480] hotel india
481: foxtrot golf [ROW:0481] hotel india
482: foxtrot golf [ROW:0482] hotel india
483: foxtrot golf [ROW:0483] hotel india
484: foxtrot golf [ROW:0484] hotel india
485: foxtrot golf [ROW:0485] hotel india
486: foxtrot golf [ROW:0486] hotel india
487: foxtrot golf [ROW:0487] hotel india
488: foxtrot golf [ROW:0488] hotel india
489: foxtrot golf [ROW:0489] hotel india
490: foxtrot golf [ROW:0490] hotel india
491: foxtrot golf [ROW:0491] hotel india
492: foxtrot golf [ROW:0492] hotel india
493: foxtrot golf [ROW:0493] hotel india
494: foxtrot golf [ROW:0494] hotel india
495: foxtrot golf [ROW:0495] hotel india
496: foxtrot golf [ROW:0496] hotel india
497: foxtrot golf [ROW:0497] hotel india
498: foxtrot golf [ROW:0498] hotel india
499: foxtrot golf [ROW:0499] hotel india
500: foxtrot golf [ROW:0500] hotel india
501: foxtrot golf [ROW:0501] hotel india
502: foxtrot golf [ROW:0502] hotel india
503: foxtrot golf [ROW:0503] hotel india
504: foxtrot golf [ROW:0504] hotel india
505: foxtrot golf [ROW:0505] hotel india
506: foxtrot golf [ROW:0506] hotel india
507: foxtrot golf [ROW:0507] hotel india
508: foxtrot golf [ROW:0508] hotel india
509: foxtrot golf [ROW:0509] hotel india
510: foxtrot golf [ROW:0510] hotel india
511: foxtrot golf [ROW:0511] hotel india
512: foxtrot golf [ROW:0512] hotel india
513: foxtrot golf [ROW:0513] hotel india
514: foxtrot golf [ROW:0514] hotel india
515: foxtrot golf [ROW:0515] hotel india
516: foxtrot golf [ROW:0516] hotel india
517: foxtrot golf [ROW:0517] hotel india
518: foxtrot golf [ROW:0518] hotel india
519: foxtrot golf [ROW:0519] hotel india
520: foxtrot golf [ROW:0520] hotel india
521: foxtrot golf [ROW:0521] hotel india
522: foxtrot golf [ROW:0522] hotel india
523: foxtrot golf [ROW:0523] hotel india
524: foxtrot golf [ROW:0524] hotel india
525: foxtrot golf [ROW:0525] hotel india
526: foxtrot golf [ROW:0526] hotel india
527: foxtrot golf [ROW:0527] hotel india
528: foxtrot golf [ROW:0528] hotel india
529: foxtrot golf [ROW:0529] hotel india
530: foxtrot golf [ROW:0530] hotel india
531: foxtrot golf [ROW:0531] hotel india
532: foxtrot golf [ROW:0532] hotel india
533: foxtrot golf [ROW:0533] hotel india
534: foxtrot golf [ROW:0534] hotel india
535: foxtrot golf [ROW:0535] hotel india
536: foxtrot golf [ROW:0536] hotel india
537: foxtrot golf [ROW:0537] hotel india
538: foxtrot golf [ROW:0538] hotel india
539: foxtrot golf [ROW:0539] hotel india
540: foxtrot golf [ROW:0540] hotel india
541: foxtrot golf [ROW:0541] hotel india
542: foxtrot golf [ROW:0542] hotel india
543: foxtrot golf [ROW:0543] hotel india
544: foxtrot golf [ROW:0544] hotel india
545: foxtrot golf [ROW:0545] hotel india
546: foxtrot golf [ROW:0546] hotel india
547: foxtrot golf [ROW:0547] hotel india
548: foxtrot golf [ROW:0548] hotel india
549: foxtrot golf [ROW:0549] hotel india
550: foxtrot golf [ROW:0550] hotel india
551: foxtrot golf [ROW:0551] hotel india
552: foxtrot golf [ROW:0552] hotel india
553: foxtrot golf [ROW:0553] hotel india
554: foxtrot golf [ROW:0554] hotel india
555: foxtrot golf [ROW:0555] hotel india
556: foxtrot golf [ROW:0556] hotel india
557: foxtrot golf [ROW:0557] hotel india
558: foxtrot golf [ROW:0558] hotel india
559: foxtrot golf [ROW:0559] hotel india
560: foxtrot golf [ROW:0560] hotel india
561: foxtrot golf [ROW:0561] hotel india
562: foxtrot golf [ROW:0562] hotel india
563: foxtrot golf [ROW:0563] hotel india
564: foxtrot golf [ROW:0564] hotel india
565: foxtrot golf [ROW:0565] hotel india
566: foxtrot golf [ROW:0566] hotel india
567: foxtrot golf [ROW:0567] hotel india
568: foxtrot golf [ROW:0568] hotel india
569: foxtrot golf [ROW:0569] hotel india
570: foxtrot golf [ROW:0570] hotel india
571: foxtrot golf [ROW:0571] hotel india
572: foxtrot golf [ROW:0572] hotel india
573: foxtrot golf [ROW:0573] hotel india
574: foxtrot golf [ROW:0574] hotel india
575: foxtrot golf [ROW:0575] hotel india
576: foxtrot golf [ROW:0576] hotel india
577: foxtrot golf [ROW:0577] hotel india
578: foxtrot golf [ROW:0578] hotel india
579: foxtrot golf [ROW:0579] hotel india
580: foxtrot golf [ROW:0580] hotel india
581: foxtrot golf [ROW:0581] hotel india
582: foxtrot golf [ROW:0582] hotel india
583: foxtrot golf [ROW:0583] hotel india
584: foxtrot golf [ROW:0584] hotel india
585: foxtrot golf [ROW:0585] hotel india
586: foxtrot golf [ROW:0586] hotel india
587: foxtrot golf [ROW:0587] hotel india
588: foxtrot golf [ROW:0588] hotel india
589: foxtrot golf [ROW:0589] hotel india
590: foxtrot golf [ROW:0590] hotel india
591: foxtrot golf [ROW:0591] hotel india
592: foxtrot golf [ROW:0592] hotel india
593: foxtrot golf [ROW:0593] hotel india
594: foxtrot golf [ROW:0594] hotel india
595: foxtrot golf [ROW:0595] hotel india
596: foxtrot golf [ROW:0596] hotel india
597: foxtrot golf [ROW:0597] hotel india
598: foxtrot golf [ROW:0598] hotel india
599: foxtrot golf [ROW:0599] hotel india
600: foxtrot golf [ROW:0600] hotel india
601: foxtrot golf [ROW:0601] hotel india
602: foxtrot golf [ROW:0602] hotel india
603: foxtrot golf [ROW:0603] hotel india
604: foxtrot golf [ROW:0604] hotel india
605: foxtrot golf [ROW:0605] hotel india
606: foxtrot golf [ROW:0606] hotel india
607: foxtrot golf [ROW:0607] hotel india
608: foxtrot golf [ROW:0608] hotel india
609: foxtrot golf [ROW:0609] hotel india
610: foxtrot golf [ROW:0610] hotel india
611: foxtrot golf [ROW:0611] hotel india
612: foxtrot golf [ROW:0612] hotel india
613: foxtrot golf [ROW:0613] hotel india
614: foxtrot golf [ROW:0614] hotel india
615: foxtrot golf [ROW:0615] hotel india
616: foxtrot golf [ROW:0616] hotel india
617: foxtrot golf [ROW:0617] hotel india
618: foxtrot golf [ROW:0618] hotel india
619: foxtrot golf [ROW:0619] hotel india
620: foxtrot golf [ROW:0620] hotel india
621: foxtrot golf [ROW:0621] hotel india
622: foxtrot golf [ROW:0622] hotel india
623: foxtrot golf [ROW:0623] hotel india
624: foxtrot golf [ROW:0624] hotel india
625: foxtrot golf [ROW:0625] hotel india
626: foxtrot golf [ROW:0626] hotel india
627: foxtrot golf [ROW:0627] hotel india
628: foxtrot golf [ROW:0628] hotel india
629: foxtrot golf [ROW:0629] hotel india
630: foxtrot golf [ROW:0630] hotel india
631: foxtrot golf [ROW:0631] hotel india
632: foxtrot golf [ROW:0632] hotel india
633: foxtrot golf [ROW:0633] hotel india
634: foxtrot golf [ROW:0634] hotel india
635: foxtrot golf [ROW:0635] hotel india
636: foxtrot golf [ROW:0636] hotel india
637: foxtrot golf [ROW:0637] hotel india
638: foxtrot golf [ROW:0638] hotel india
639: foxtrot golf [ROW:0639] hotel india
640: foxtrot golf [ROW:0640] hotel india
641: foxtrot golf [ROW:0641] hotel india
642: foxtrot golf [ROW:0642] hotel india
643: foxtrot golf [ROW:0643] hotel india
644: foxtrot golf [ROW:0644] hotel india
645: foxtrot golf [ROW:0645] hotel india
646: foxtrot golf [ROW:0646] hotel india
647: foxtrot golf [ROW:0647] hotel india
648: foxtrot golf [ROW:0648] hotel india
649: foxtrot golf [ROW:0649] hotel india
650: foxtrot golf [ROW:0650] hotel india
651: foxtrot golf [ROW:0651] hotel india
652: foxtrot golf [ROW:0652] hotel india
653: foxtrot golf [ROW:0653] hotel india
654: foxtrot golf [ROW:0654] hotel india
655: foxtrot golf [ROW:0655] hotel india
656: foxtrot golf [ROW:0656] hotel india
657: foxtrot golf [ROW:0657] hotel india
658: foxtrot golf [ROW:0658] hotel india
659: foxtrot golf [ROW:0659] hotel india
660: foxtrot golf [ROW:0660] hotel india
661: foxtrot golf [ROW:0661] hotel india
662: foxtrot golf [ROW:0662] hotel india
663: foxtrot golf [ROW:0663] hotel india
664: foxtrot golf [ROW:0664] hotel india
665: foxtrot golf [ROW:0665] hotel india
666: foxtrot golf [ROW:0666] hotel india
667: foxtrot golf [ROW:0667] hotel india
668: foxtrot golf [ROW:0668] hotel india
669: foxtrot golf [ROW:0669] hotel india
670: foxtrot golf [ROW:0670] hotel india
671: foxtrot golf [ROW:0671] hotel india
672: foxtrot golf [ROW:0672] hotel india
673: foxtrot golf [ROW:0673] hotel india
674: foxtrot golf [ROW:0674] hotel india
675: foxtrot golf [ROW:0675] hotel india
676: foxtrot golf [ROW:0676] hotel india
677: foxtrot golf [ROW:0677] hotel india
678: foxtrot golf [ROW:0678] hotel india
679: foxtrot golf [ROW:0679] hotel india
680: foxtrot golf [ROW:0680] hotel india
681: foxtrot golf [ROW:0681] hotel india
682: foxtrot golf [ROW:0682] hotel india
683: foxtrot golf [ROW:0683] hotel india
684: foxtrot golf [ROW:0684] hotel india
685: foxtrot golf [ROW:0685] hotel india
686: foxtrot golf [ROW:0686] hotel india
687: foxtrot golf [ROW:0687] hotel india
688: foxtrot golf [ROW:0688] hotel india
689: foxtrot golf [ROW:0689] hotel india
690: foxtrot golf [ROW:0690] hotel india
691: foxtrot golf [ROW:0691] hotel india
692: foxtrot golf [ROW:0692] hotel india
693: foxtrot golf [ROW:0693] hotel india
694: foxtrot golf [ROW:0694] hotel india
695: foxtrot golf [ROW:0695] hotel india
696: foxtrot golf [ROW:0696] hotel india
697: foxtrot golf [ROW:0697] hotel india
698: foxtrot golf [ROW:0698] hotel india
699: foxtrot golf [ROW:0699] hotel india
700: foxtrot golf [ROW:0700] hotel india
701: foxtrot golf [ROW:0701] hotel india
702: foxtrot golf [ROW:0702] hotel india
703: foxtrot golf [ROW:0703] hotel india
704: foxtrot golf [ROW:0704] hotel india
705: foxtrot golf [ROW:0705] hotel india
706: foxtrot golf [ROW:0706] hotel india
707: foxtrot golf [ROW:0707] hotel india
708: foxtrot golf [ROW:0708] hotel india
709: foxtrot golf [ROW:0709] hotel india
710: foxtrot golf [ROW:0710] hotel india
711: foxtrot golf [ROW:0711] hotel india
712: foxtrot golf [ROW:0712] hotel india
713: foxtrot golf [ROW:0713] hotel india
714: foxtrot golf [ROW:0714] hotel india
715: foxtrot golf [ROW:0715] hotel india
716: foxtrot golf [ROW:0716] hotel india
717: foxtrot golf [ROW:0717] hotel india
718: foxtrot golf [ROW:0718] hotel india
719: foxtrot golf [ROW:0719] hotel india
720: foxtrot golf [ROW:0720] hotel india
721: foxtrot golf [ROW:0721] hotel india
722: foxtrot golf [ROW:0722] hotel india
723: foxtrot golf [ROW:0723] hotel india
724: foxtrot golf [ROW:0724] hotel india
725: foxtrot golf [ROW:0725] hotel india
726: foxtrot golf [ROW:0726] hotel india
727: foxtrot golf [ROW:0727] hotel india
728: foxtrot golf [ROW:0728] hotel india
729: foxtrot golf [ROW:0729] hotel india
730: foxtrot golf [ROW:0730] hotel india
731: foxtrot golf [ROW:0731] hotel india
732: foxtrot golf [ROW:0732] hotel india
733: foxtrot golf [ROW:0733] hotel india
734: foxtrot golf [ROW:0734] hotel india
735: foxtrot golf [ROW:0735] hotel india
736: foxtrot golf [ROW:0736] hotel india
737: foxtrot golf [ROW:0737] hotel india
738: foxtrot golf [ROW:0738] hotel india
739: foxtrot golf [ROW:0739] hotel india
740: foxtrot golf [ROW:0740] hotel india
741: foxtrot golf [ROW:0741] hotel india
742: foxtrot golf [ROW:0742] hotel india
743: foxtrot golf [ROW:0743] hotel india
744: foxtrot golf [ROW:0744] hotel india
745: foxtrot golf [ROW:0745] hotel india
746: foxtrot golf [ROW:0746] hotel india
747: foxtrot golf [ROW:0747] hotel india
748: foxtrot golf [ROW:0748] hotel india
749: foxtrot golf [ROW:0749] hotel india
750: foxtrot golf [ROW:0750] hotel india
751: foxtrot golf [ROW:0751] hotel india
752: foxtrot golf [ROW:0752] hotel india
753: foxtrot golf [ROW:0753] hotel india
754: foxtrot golf [ROW:0754] hotel india
755: foxtrot golf [ROW:0755] hotel india
756: foxtrot golf [ROW:0756] hotel india
757: foxtrot golf [ROW:0757] hotel india
758: foxtrot golf [ROW:0758] hotel india
759: foxtrot golf [ROW:0759] hotel india
760: foxtrot golf [ROW:0760] hotel india
761: foxtrot golf [ROW:0761] hotel india
762: foxtrot golf [ROW:0762] hotel india
763: foxtrot golf [ROW:0763] hotel india
764: foxtrot golf [ROW:0764] hotel india
765: foxtrot golf [ROW:0765] hotel india
766: foxtrot golf [ROW:0766] hotel india
767: foxtrot golf [ROW:0767] hotel india
768: foxtrot golf [ROW:0768] hotel india
769: foxtrot golf [ROW:0769] hotel india
770: foxtrot golf [ROW:0770] hotel india
771: foxtrot golf [ROW:0771] hotel india
772: foxtrot golf [ROW:0772] hotel india
773: foxtrot golf [ROW:0773] hotel india
774: foxtrot golf [ROW:0774] hotel india
775: foxtrot golf [ROW:0775] hotel india
776: foxtrot golf [ROW:0776] hotel india
777: foxtrot golf [ROW:0777] hotel india
778: foxtrot golf [ROW:0778] hotel india
779: foxtrot golf [ROW:0779] hotel india
780: foxtrot golf [ROW:0780] hotel india
781: foxtrot golf [ROW:0781] hotel india
782: foxtrot golf [ROW:0782] hotel india
783: foxtrot golf [ROW:0783] hotel india
784: foxtrot golf [ROW:0784] hotel india
785: foxtrot golf [ROW:0785] hotel india
786: foxtrot golf [ROW:0786] hotel india
787: foxtrot golf [ROW:0787] hotel india
788: foxtrot golf [ROW:0788] hotel india
789: foxtrot golf [ROW:0789] hotel india
790: foxtrot golf [ROW:0790] hotel india
791: foxtrot golf [ROW:0791] hotel india
792: foxtrot golf [ROW:0792] hotel india
793: foxtrot golf [ROW:0793] hotel india
794: foxtrot golf [ROW:0794] hotel india
795: foxtrot golf [ROW:0795] hotel india
796: foxtrot golf [ROW:0796] hotel india
797: foxtrot golf [ROW:0797] hotel india
798: foxtrot golf [ROW:0798] hotel india
799: foxtrot golf [ROW:0799] hotel india
800: foxtrot golf [ROW:0800] hotel india
801: foxtrot golf [ROW:0801] hotel india
802: foxtrot golf [ROW:0802] hotel india
803: foxtrot golf [ROW:0803] hotel india
804: foxtrot golf [ROW:0804] hotel india
805: foxtrot golf [ROW:0805] hotel india
806: foxtrot golf [ROW:0806] hotel india
807: foxtrot golf [ROW:0807] hotel india
808: foxtrot golf [ROW:0808] hotel india
809: foxtrot golf [ROW:0809] hotel india
810: foxtrot golf [ROW:0810] hotel india
811: foxtrot golf [ROW:0811] hotel india
812: foxtrot golf [ROW:0812] hotel india
813: foxtrot golf [ROW:0813] hotel india
814: foxtrot golf [ROW:0814] hotel india
815: foxtrot golf [ROW:0815] hotel india
816: foxtrot golf [ROW:0816] hotel india
817: foxtrot golf [ROW:0817] hotel india
818: foxtrot golf [ROW:0818] hotel india
819: foxtrot golf [ROW:0819] hotel india
820: foxtrot golf [ROW:0820] hotel india
821: foxtrot golf [ROW:0821] hotel india
822: foxtrot golf [ROW:0822] hotel india
823: foxtrot golf [ROW:0823] hotel india
824: foxtrot golf [ROW:0824] hotel india
825: foxtrot golf [ROW:0825] hotel india
826: foxtrot golf [ROW:0826] hotel india
827: foxtrot golf [ROW:0827] hotel india
828: foxtrot golf [ROW:0828] hotel india
829: foxtrot golf [ROW:0829] hotel india
830: foxtrot golf [ROW:0830] hotel india
831: foxtrot golf [ROW:0831] hotel india
832: foxtrot golf [ROW:0832] hotel india
833: foxtrot golf [ROW:0833] hotel india
834: foxtrot golf [ROW:0834] hotel india
835: foxtrot golf [ROW:0835] hotel india
836: foxtrot golf [ROW:0836] hotel india
837: foxtrot golf [ROW:0837] hotel india
838: foxtrot golf [ROW:0838] hotel india
839: foxtrot golf [ROW:0839] hotel india
840: foxtrot golf [ROW:0840] hotel india
841: foxtrot golf [ROW:0841] hotel india
842: foxtrot golf [ROW:0842] hotel india
843: foxtrot golf [ROW:0843] hotel india
844: foxtrot golf [ROW:0844] hotel india
845: foxtrot golf [ROW:0845] hotel india
846: foxtrot golf [ROW:0846] hotel india
847: foxtrot golf [ROW:0847] hotel india
848: foxtrot golf [ROW:0848] hotel india
849: foxtrot golf [ROW:0849] hotel india
850: foxtrot golf [ROW:0850] hotel india
851: foxtrot golf [ROW:0851] hotel india
852: foxtrot golf [ROW:0852] hotel india
853: foxtrot golf [ROW:0853] hotel india
854: foxtrot golf [ROW:0854] hotel india
855: foxtrot golf [ROW:0855] hotel india
856: foxtrot golf [ROW:0856] hotel india
857: foxtrot golf [ROW:0857] hotel india
858: foxtrot golf [ROW:0858] hotel india
859: foxtrot golf [ROW:0859] hotel india
860: foxtrot golf [ROW:0860] hotel india
861: foxtrot golf [ROW:0861] hotel india
862: foxtrot golf [ROW:0862] hotel india
863: foxtrot golf [ROW:0863] hotel india
864: foxtrot golf [ROW:0864] hotel india
865: foxtrot golf [ROW:0865] hotel india
866: foxtrot golf [ROW:0866] hotel india
867: foxtrot golf [ROW:0867] hotel india
868: foxtrot golf [ROW:0868] hotel india
869: foxtrot golf [ROW:0869] hotel india
870: foxtrot golf [ROW:0870] hotel india
871: foxtrot golf [ROW:0871] hotel india
872: foxtrot golf [ROW:0872] hotel india
873: foxtrot golf [ROW:0873] hotel india
874: foxtrot golf [ROW:0874] hotel india
875: foxtrot golf [ROW:0875] hotel india
876: foxtrot golf [ROW:0876] hotel india
877: foxtrot golf [ROW:0877] hotel india
878: foxtrot golf [ROW:0878] hotel india
879: foxtrot golf [ROW:0879] hotel india
880: foxtrot golf [ROW:0880] hotel india
881: foxtrot golf [ROW:0881] hotel india
882: foxtrot golf [ROW:0882] hotel india
883: foxtrot golf [ROW:0883] hotel india
884: foxtrot golf [ROW:0884] hotel india
885: foxtrot golf [ROW:0885] hotel india
886: foxtrot golf [ROW:0886] hotel india
887: foxtrot golf [ROW:0887] hotel india
888: foxtrot golf [ROW:0888] hotel india
889: foxtrot golf [ROW:0889] hotel india
890: foxtrot golf [ROW:0890] hotel india
891: foxtrot golf [ROW:0891] hotel india
892: foxtrot golf [ROW:0892] hotel india
893: foxtrot golf [ROW:0893] hotel india
894: foxtrot golf [ROW:0894] hotel india
895: foxtrot golf [ROW:0895] hotel india
896: foxtrot golf [ROW:0896] hotel india
897: foxtrot golf [ROW:0897] hotel india
898: foxtrot golf [ROW:0898] hotel india
899: foxtrot golf [ROW:0899] hotel india
900: foxtrot golf [ROW:0900] hotel india
901: foxtrot golf [ROW:0901] hotel india
902: foxtrot golf [ROW:0902] hotel india
903: foxtrot golf [ROW:0903] hotel india
904: foxtrot golf [ROW:0904] hotel india
905: foxtrot golf [ROW:0905] hotel india
906: foxtrot golf [ROW:0906] hotel india
907: foxtrot golf [ROW:0907] hotel india
908: foxtrot golf [ROW:0908] hotel india
909: foxtrot golf [ROW:0909] hotel india
910: foxtrot golf [ROW:0910] hotel india
911: foxtrot golf [ROW:0911] hotel india
912: foxtrot golf [ROW:0912] hotel india
913: foxtrot golf [ROW:0913] hotel india
914: foxtrot golf [ROW:0914] hotel india
915: foxtrot golf [ROW:0915] hotel india
916: foxtrot golf [ROW:0916] hotel india
917: foxtrot golf [ROW:0917] hotel india
918: foxtrot golf [ROW:0918] hotel india
919: foxtrot golf [ROW:0919] hotel india
920: foxtrot golf [ROW:0920] hotel india
921: foxtrot golf [ROW:0921] hotel india
922: foxtrot golf [ROW:0922] hotel india
923: foxtrot golf [ROW:0923] hotel india
924: foxtrot golf [ROW:0924] hotel india
925: foxtrot golf [ROW:0925] hotel india
926: foxtrot golf [ROW:0926] hotel india
927: foxtrot golf [ROW:0927] hotel india
928: foxtrot golf [ROW:0928] hotel india
929: foxtrot golf [ROW:0929] hotel india
930: foxtrot golf [ROW:0930] hotel india
931: foxtrot golf [ROW:0931] hotel india
932: foxtrot golf [ROW:0932] hotel india
933: foxtrot golf [ROW:0933] hotel india
934: foxtrot golf [ROW:0934] hotel india
935: foxtrot golf [ROW:0935] hotel india
936: foxtrot golf [ROW:0936] hotel india
937: foxtrot golf [ROW:0937] hotel india
938: foxtrot golf [ROW:0938] hotel india
939: foxtrot golf [ROW:0939] hotel india
940: foxtrot golf [ROW:0940] hotel india
941: foxtrot golf [ROW:0941] hotel india
942: foxtrot golf [ROW:0942] hotel india
943: foxtrot golf [ROW:0943] hotel india
944: foxtrot golf [ROW:0944] hotel india
945: foxtrot golf [ROW:0945] hotel india
946: foxtrot golf [ROW:0946] hotel india
947: foxtrot golf [ROW:0947] hotel india
948: foxtrot golf [ROW:0948] hotel india
949: foxtrot golf [ROW:0949] hotel india
950: foxtrot golf [ROW:0950] hotel india
951: foxtrot golf [ROW:0951] hotel india
952: foxtrot golf [ROW:0952] hotel india
953: foxtrot golf [ROW:0953] hotel india
954: foxtrot golf [ROW:0954] hotel india
955: foxtrot golf [ROW:0955] hotel india
956: foxtrot golf [ROW:0956] hotel india
957: foxtrot golf [ROW:0957] hotel india
958: foxtrot golf [ROW:0958] hotel india
959: foxtrot golf [ROW:0959] hotel india
960: foxtrot golf [ROW:0960] hotel india
961: foxtrot golf [ROW:0961] hotel india
962: foxtrot golf [ROW:0962] hotel india
963: foxtrot golf [ROW:0963] hotel india
964: foxtrot golf [ROW:0964] hotel india
965: foxtrot golf [ROW:0965] hotel india
966: foxtrot golf [ROW:0966] hotel india
967: foxtrot golf [ROW:0967] hotel india
968: foxtrot golf [ROW:0968] hotel india
969: foxtrot golf [ROW:0969] hotel india
970: foxtrot golf [ROW:0970] hotel india
971: foxtrot golf [ROW:0971] hotel india
972: foxtrot golf [ROW:0972] hotel india
973: foxtrot golf [ROW:0973] hotel india
974: foxtrot golf [ROW:0974] hotel india
975: foxtrot golf [ROW:0975] hotel india
976: foxtrot golf [ROW:0976] hotel india
977: foxtrot golf [ROW:0977] hotel india
978: foxtrot golf [ROW:0978] hotel india
979: foxtrot golf [ROW:0979] hotel india
980: foxtrot golf [ROW:0980] hotel india
981: foxtrot golf [ROW:0981] hotel india
982: foxtrot golf [ROW:0982] hotel india
983: foxtrot golf [ROW:0983] hotel india
984: foxtrot golf [ROW:0984] hotel india
985: foxtrot golf [ROW:0985] hotel india
986: foxtrot golf [ROW:0986] hotel india
987: foxtrot golf [ROW:0987] hotel india
988: foxtrot golf [ROW:0988] hotel india
989: foxtrot golf [ROW:0989] hotel india
990: foxtrot golf [ROW:0990] hotel india
991: foxtrot golf [ROW:0991] hotel india
992: foxtrot golf [ROW:0992] hotel india
993: foxtrot golf [ROW:0993] hotel india
994: foxtrot golf [ROW:0994] hotel india
995: foxtrot golf [ROW:0995] hotel india
996: foxtrot golf [ROW:0996] hotel india
997: foxtrot golf [ROW:0997] hotel india
998: foxtrot golf [ROW:0998] hotel india
999: foxtrot golf [ROW:0999] hotel india
1000: foxtrot golf [ROW:1000] hotel india
1001: foxtrot golf [ROW:1001] hotel india
1002: foxtrot golf [ROW:1002] hotel india
1003: foxtrot golf [ROW:1003] hotel india
1004: foxtrot golf [ROW:1004] hotel india
1005: foxtrot golf [ROW:1005] hotel india
1006: foxtrot golf [ROW:1006] hotel india
1007: foxtrot golf [ROW:1007] hotel india
1008: foxtrot golf [ROW:1008] hotel india
1009: foxtrot golf [ROW:1009] hotel india
1010: foxtrot golf [ROW:1010] hotel india
1011: foxtrot golf [ROW:1011] hotel india
1012: foxtrot golf [ROW:1012] hotel india
1013: foxtrot golf [ROW:1013] hotel india
1014: foxtrot golf [ROW:1014] hotel india
1015: foxtrot golf [ROW:1015] hotel india
1016: foxtrot golf [ROW:1016] hotel india
1017: foxtrot golf [ROW:1017] hotel india
1018: foxtrot golf [ROW:1018] hotel india
1019: foxtrot golf [ROW:1019] hotel india
1020: foxtrot golf [ROW:1020] hotel india
1021: foxtrot golf [ROW:1021] hotel india
1022: foxtrot golf [ROW:1022] hotel india
1023: foxtrot golf [ROW:1023] hotel india
1024: foxtrot golf [ROW:1024] hotel india
1025: foxtrot golf [ROW:1025] hotel india
1026: foxtrot golf [ROW:1026] hotel india
1027: foxtrot golf [ROW:1027] hotel india
1028: foxtrot golf [ROW:1028] hotel india
1029: foxtrot golf [ROW:1029] hotel india
1030: foxtrot golf [ROW:1030] hotel india
1031: foxtrot golf [ROW:1031] hotel india
1032: foxtrot golf [ROW:1032] hotel india
1033: foxtrot golf [ROW:1033] hotel india
1034: foxtrot golf [ROW:1034] hotel india
1035: foxtrot golf [ROW:1035] hotel india
1036: foxtrot golf [ROW:1036] hotel india
1037: foxtrot golf [ROW:1037] hotel india
1038: foxtrot golf [ROW:1038] hotel india
1039: foxtrot golf [ROW:1039] hotel india
1040: foxtrot golf [ROW:1040] hotel india
1041: foxtrot golf [ROW:1041] hotel india
1042: foxtrot golf [ROW:1042] hotel india
1043: foxtrot golf [ROW:1043] hotel india
1044: foxtrot golf [ROW:1044] hotel india
1045: foxtrot golf [ROW:1045] hotel india
1046: foxtrot golf [ROW:1046] hotel india
1047: foxtrot golf [ROW:1047] hotel india
1048: foxtrot golf [ROW:1048] hotel india
1049: foxtrot golf [ROW:1049] hotel india
1050: foxtrot golf [ROW:1050] hotel india
1051: foxtrot golf [ROW:1051] hotel india
1052: foxtrot golf [ROW:1052] hotel india
1053: foxtrot golf [ROW:1053] hotel india
1054: foxtrot golf [ROW:1054] hotel india
1055: foxtrot golf [ROW:1055] hotel india
1056: foxtrot golf [ROW:1056] hotel india
1057: foxtrot golf [ROW:1057] hotel india
1058: foxtrot golf [ROW:1058] hotel india
1059: foxtrot golf [ROW:1059] hotel india
1060: foxtrot golf [ROW:1060] hotel india
1061: foxtrot golf [ROW:1061] hotel india
1062: foxtrot golf [ROW:1062] hotel india
1063: foxtrot golf [ROW:1063] hotel india
1064: foxtrot golf [ROW:1064] hotel india
1065: foxtrot golf [ROW:1065] hotel india
1066: foxtrot golf [ROW:1066] hotel india
1067: foxtrot golf [ROW:1067] hotel india
1068: foxtrot golf [ROW:1068] hotel india
1069: foxtrot golf [ROW:1069] hotel india
1070: foxtrot golf [ROW:1070] hotel india
1071: foxtrot golf [ROW:1071] hotel india
1072: foxtrot golf [ROW:1072] hotel india
1073: foxtrot golf [ROW:1073] hotel india
1074: foxtrot golf [ROW:1074] hotel india
1075: foxtrot golf [ROW:1075] hotel india
1076: foxtrot golf [ROW:1076] hotel india
1077: foxtrot golf [ROW:1077] hotel india
1078: foxtrot golf [ROW:1078] hotel india
1079: foxtrot golf [ROW:1079] hotel india
1080: foxtrot golf [ROW:1080] hotel india
1081: foxtrot golf [ROW:1081] hotel india
1082: foxtrot golf [ROW:1082] hotel india
1083: foxtrot golf [ROW:1083] hotel india
1084: foxtrot golf [ROW:1084] hotel india
1085: foxtrot golf [ROW:1085] hotel india
1086: foxtrot golf [ROW:1086] hotel india
1087: foxtrot golf [ROW:1087] hotel india
1088: foxtrot golf [ROW:1088] hotel india
1089: foxtrot golf [ROW:1089] hotel india
1090: foxtrot golf [ROW:1090] hotel india
1091: foxtrot golf [ROW:1091] hotel india
1092: foxtrot golf [ROW:1092] hotel india
1093: foxtrot golf [ROW:1093] hotel india
1094: foxtrot golf [ROW:1094] hotel india
1095: foxtrot golf [ROW:1095] hotel india
1096: foxtrot golf [ROW:1096] hotel india
1097: foxtrot golf [ROW:1097] hotel india
1098: foxtrot golf [ROW:1098] hotel india
1099: foxtrot golf [ROW:1099] hotel india
1100: foxtrot golf [ROW:1100] hotel india
1101: foxtrot golf [ROW:1101] hotel india
1102: foxtrot golf [ROW:1102] hotel india
1103: foxtrot golf [ROW:1103] hotel india
1104: foxtrot golf [ROW:1104] hotel india
1105: foxtrot golf [ROW:1105] hotel india
1106: foxtrot golf [ROW:1106] hotel india
1107: foxtrot golf [ROW:1107] hotel india
1108: foxtrot golf [ROW:1108] hotel india
1109: foxtrot golf [ROW:1109] hotel india
1110: foxtrot golf [ROW:1110] hotel india
1111: foxtrot golf [ROW:1111] hotel india
1112: foxtrot golf [ROW:1112] hotel india
1113: foxtrot golf [ROW:1113] hotel india
1114: foxtrot golf [ROW:1114] hotel india
1115: foxtrot golf [ROW:1115] hotel india
1116: foxtrot golf [ROW:1116] hotel india
1117: foxtrot golf [ROW:1117] hotel india
1118: foxtrot golf [ROW:1118] hotel india
1119: foxtrot golf [ROW:1119] hotel india
1120: foxtrot golf [ROW:1120] hotel india
1121: foxtrot golf [ROW:1121] hotel india
1122: foxtrot golf [ROW:1122] hotel india
1123: foxtrot golf [ROW:1123] hotel india
1124: foxtrot golf [ROW:1124] hotel india
1125: foxtrot golf [ROW:1125] hotel india
1126: foxtrot golf [ROW:1126] hotel india
1127: foxtrot golf [ROW:1127] hotel india
1128: foxtrot golf [ROW:1128] hotel india
1129: foxtrot golf [ROW:1129] hotel india
1130: foxtrot golf [ROW:1130] hotel india
1131: foxtrot golf [ROW:1131] hotel india
1132: foxtrot golf [ROW:1132] hotel india
1133: foxtrot golf [ROW:1133] hotel india
1134: foxtrot golf [ROW:1134] hotel india
1135: foxtrot golf [ROW:1135] hotel india
1136: foxtrot golf [ROW:1136] hotel india
1137: foxtrot golf [ROW:1137] hotel india
1138: foxtrot golf [ROW:1138] hotel india
1139: foxtrot golf [ROW:1139] hotel india
1140: foxtrot golf [ROW:1140] hotel india
1141: foxtrot golf [ROW:1141] hotel india
1142: foxtrot golf [ROW:1142] hotel india
1143: foxtrot golf [ROW:1143] hotel india
1144: foxtrot golf [ROW:1144] hotel india
1145: foxtrot golf [ROW:1145] hotel india
1146: foxtrot golf [ROW:1146] hotel india
1147: foxtrot golf [ROW:1147] hotel india
1148: foxtrot golf [ROW:1148] hotel india
1149: foxtrot golf [ROW:1149] hotel india
1150: foxtrot golf [ROW:1150] hotel india
1151: foxtrot golf [ROW:1151] hotel india
1152: foxtrot golf [ROW:1152] hotel india
1153: foxtrot golf [ROW:1153] hotel india
1154: foxtrot golf [ROW:1154] hotel india
1155: foxtrot golf [ROW:1155] hotel india
1156: foxtrot golf [ROW:1156] hotel india
1157: foxtrot golf [ROW:1157] hotel india
1158: foxtrot golf [ROW:1158] hotel india
1159: foxtrot golf [ROW:1159] hotel india
1160: foxtrot golf [ROW:1160] hotel india
1161: foxtrot golf [ROW:1161] hotel india
1162: foxtrot golf [ROW:1162] hotel india
1163: foxtrot golf [ROW:1163] hotel india
1164: foxtrot golf [ROW:1164] hotel india
1165: foxtrot golf [ROW:1165] hotel india
1166: foxtrot golf [ROW:1166] hotel india
1167: foxtrot golf [ROW:1167] hotel india
1168: foxtrot golf [ROW:1168] hotel india
1169: foxtrot golf [ROW:1169] hotel india
1170: foxtrot golf [ROW:1170] hotel india
1171: foxtrot golf [ROW:1171] hotel india
1172: foxtrot golf [ROW:1172] hotel india
1173: foxtrot golf [ROW:1173] hotel india
1174: foxtrot golf [ROW:1174] hotel india
1175: foxtrot golf [ROW:1175] hotel india
1176: foxtrot golf [ROW:1176] hotel india
1177: foxtrot golf [ROW:1177] hotel india
1178: foxtrot golf [ROW:1178] hotel india
1179: foxtrot golf [ROW:1179] hotel india
1180: foxtrot golf [ROW:1180] hotel india
1181: foxtrot golf [ROW:1181] hotel india
1182: foxtrot golf [ROW:1182] hotel india
1183: foxtrot golf [ROW:1183] hotel india
1184: foxtrot golf [ROW:1184] hotel india
1185: foxtrot golf [ROW:1185] hotel india
1186: foxtrot golf [ROW:1186] hotel india
1187: foxtrot golf [ROW:1187] hotel india
1188: foxtrot golf [ROW:1188] hotel india
1189: foxtrot golf [ROW:1189] hotel india
1190: foxtrot golf [ROW:1190] hotel india
1191: foxtrot golf [ROW:1191] hotel india
1192: foxtrot golf [ROW:1192] hotel india
1193: foxtrot golf [ROW:1193] hotel india
1194: foxtrot golf [ROW:1194] hotel india
1195: foxtrot golf [ROW:1195] hotel india
1196: foxtrot golf [ROW:1196] hotel india
1197: foxtrot golf [ROW:1197] hotel india
1198: foxtrot golf [ROW:1198] hotel india
1199: foxtrot golf [ROW:1199] hotel india
1200: foxtrot golf [ROW:1200] hotel india
1201: foxtrot golf [ROW:1201] hotel india
1202: foxtrot golf [ROW:1202] hotel india
1203: foxtrot golf [ROW:1203] hotel india
1204: foxtrot golf [ROW:1204] hotel india
1205: foxtrot golf [ROW:1205] hotel india
1206: foxtrot golf [ROW:1206] hotel india
1207: foxtrot golf [ROW:1207] hotel india
1208: foxtrot golf [ROW:1208] hotel india
1209: foxtrot golf [ROW:1209] hotel india
1210: foxtrot golf [ROW:1210] hotel india
1211: foxtrot golf [ROW:1211] hotel india
1212: foxtrot golf [ROW:1212] hotel india
1213: foxtrot golf [ROW:1213] hotel india
1214: foxtrot golf [ROW:1214] hotel india
1215: foxtrot golf [ROW:1215] hotel india
1216: foxtrot golf [ROW:1216] hotel india
1217: foxtrot golf [ROW:1217] hotel india
1218: foxtrot golf [ROW:1218] hotel india
1219: foxtrot golf [ROW:1219] hotel india
1220: foxtrot golf [ROW:1220] hotel india
1221: foxtrot golf [ROW:1221] hotel india
1222: foxtrot golf [ROW:1222] hotel india
1223: foxtrot golf [ROW:1223] hotel india
1224: foxtrot golf [ROW:1224] hotel india
1225: foxtrot golf [ROW:1225] hotel india
1226: foxtrot golf [ROW:1226] hotel india
1227: foxtrot golf [ROW:1227] hotel india
1228: foxtrot golf [ROW:1228] hotel india
1229: foxtrot golf [ROW:1229] hotel india
1230: foxtrot golf [ROW:1230] hotel india
1231: foxtrot golf [ROW:1231] hotel india
1232: foxtrot golf [ROW:1232] hotel india
1233: foxtrot golf [ROW:1233] hotel india
1234: foxtrot golf [ROW:1234] hotel india
1235: foxtrot golf [ROW:1235] hotel india
1236: foxtrot golf [ROW:1236] hotel india
1237: foxtrot golf [ROW:1237] hotel india
1238: foxtrot golf [ROW:1238] hotel india
1239: foxtrot golf [ROW:1239] hotel india
1240: foxtrot golf [ROW:1240] hotel india
1241: foxtrot golf [ROW:1241] hotel india
1242: foxtrot golf [ROW:1242] hotel india
1243: foxtrot golf [ROW:1243] hotel india
1244: foxtrot golf [ROW:1244] hotel india
1245: foxtrot golf [ROW:1245] hotel india
1246: foxtrot golf [ROW:1246] hotel india
1247: foxtrot golf [ROW:1247] hotel india
1248: foxtrot golf [ROW:1248] hotel india
1249: foxtrot golf [ROW:1249] hotel india
1250: foxtrot golf [ROW:1250] hotel india
1251: foxtrot golf [ROW:1251] hotel india
1252: foxtrot golf [ROW:1252] hotel india
1253: foxtrot golf [ROW:1253] hotel india
1254: foxtrot golf [ROW:1254] hotel india
1255: foxtrot golf [ROW:1255] hotel india
1256: foxtrot golf [ROW:1256] hotel india
1257: foxtrot golf [ROW:1257] hotel india
1258: foxtrot golf [ROW:1258] hotel india
1259: foxtrot golf [ROW:1259] hotel india
1260: foxtrot golf [ROW:1260] hotel india
1261: foxtrot golf [ROW:1261] hotel india
1262: foxtrot golf [ROW:1262] hotel india
1263: foxtrot golf [ROW:1263] hotel india
1264: foxtrot golf [ROW:1264] hotel india
1265: foxtrot golf [ROW:1265] hotel india
1266: foxtrot golf [ROW:1266] hotel india
1267: foxtrot golf [ROW:1267] hotel india
1268: foxtrot golf [ROW:1268] hotel india
1269: foxtrot golf [ROW:1269] hotel india
1270: foxtrot golf [ROW:1270] hotel india
1271: foxtrot golf [ROW:1271] hotel india
1272: foxtrot golf [ROW:1272] hotel india
1273: foxtrot golf [ROW:1273] hotel india
1274: foxtrot golf [ROW:1274] hotel india
1275: foxtrot golf [ROW:1275] hotel india
1276: foxtrot golf [ROW:1276] hotel india
1277: foxtrot golf [ROW:1277] hotel india
1278: foxtrot golf [ROW:1278] hotel india
1279: foxtrot golf [ROW:1279] hotel india
1280: foxtrot golf [ROW:1280] hotel india
1281: foxtrot golf [ROW:1281] hotel india
1282: foxtrot golf [ROW:1282] hotel india
1283: foxtrot golf [ROW:1283] hotel india
1284: foxtrot golf [ROW:1284] hotel india
1285: foxtrot golf [ROW:1285] hotel india
1286: foxtrot golf [ROW:1286] hotel india
1287: foxtrot golf [ROW:1287] hotel india
1288: foxtrot golf [ROW:1288] hotel india
1289: foxtrot golf [ROW:1289] hotel india
1290: foxtrot golf [ROW:1290] hotel india
1291: foxtrot golf [ROW:1291] hotel india
1292: foxtrot golf [ROW:1292] hotel india
1293: foxtrot golf [ROW:1293] hotel india
1294: foxtrot golf [ROW:1294] hotel india
1295: foxtrot golf [ROW:1295] hotel india
1296: foxtrot golf [ROW:1296] hotel india
1297: foxtrot golf [ROW:1297] hotel india
1298: foxtrot golf [ROW:1298] hotel india
1299: foxtrot golf [ROW:1299] hotel india
1300: foxtrot golf [ROW:1300] hotel india
1301: foxtrot golf [ROW:1301] hotel india
1302: foxtrot golf [ROW:1302] hotel india
1303: foxtrot golf [ROW:1303] hotel india
1304: foxtrot golf [ROW:1304] hotel india
1305: foxtrot golf [ROW:1305] hotel india
1306: foxtrot golf [ROW:1306] hotel india
1307: foxtrot golf [ROW:1307] hotel india
1308: foxtrot golf [ROW:1308] hotel india
1309: foxtrot golf [ROW:1309] hotel india
1310: foxtrot golf [ROW:1310] hotel india
1311: foxtrot golf [ROW:1311] hotel india
1312: foxtrot golf [ROW:1312] hotel india
1313: foxtrot golf [ROW:1313] hotel india
1314: foxtrot golf [ROW:1314] hotel india
1315: foxtrot golf [ROW:1315] hotel india
1316: foxtrot golf [ROW:1316] hotel india
1317: foxtrot golf [ROW:1317] hotel india
1318: foxtrot golf [ROW:1318] hotel india
1319: foxtrot golf [ROW:1319] hotel india
1320: foxtrot golf [ROW:1320] hotel india
1321: foxtrot golf [ROW:1321] hotel india
1322: foxtrot golf [ROW:1322] hotel india
1323: foxtrot golf [ROW:1323] hotel india
1324: foxtrot golf [ROW:1324] hotel india
1325: foxtrot golf [ROW:1325] hotel india
1326: foxtrot golf [ROW:1326] hotel india
1327: foxtrot golf [ROW:1327] hotel india
1328: foxtrot golf [ROW:1328] hotel india
1329: foxtrot golf [ROW:1329] hotel india
1330: foxtrot golf [ROW:1330] hotel india
1331: foxtrot golf [ROW:1331] hotel india
1332: foxtrot golf [ROW:1332] hotel india
1333: foxtrot golf [ROW:1333] hotel india
1334: foxtrot golf [ROW:1334] hotel india
1335: foxtrot golf [ROW:1335] hotel india
1336: foxtrot golf [ROW:1336] hotel india
1337: foxtrot golf [ROW:1337] hotel india
1338: foxtrot golf [ROW:1338] hotel india
1339: foxtrot golf [ROW:1339] hotel india
1340: foxtrot golf [ROW:1340] hotel india
1341: foxtrot golf [ROW:1341] hotel india
1342: foxtrot golf [ROW:1342] hotel india
1343: foxtrot golf [ROW:1343] hotel india
1344: foxtrot golf [ROW:1344] hotel india
1345: foxtrot golf [ROW:1345] hotel india
1346: foxtrot golf [ROW:1346] hotel india
1347: foxtrot golf [ROW:1347] hotel india
1348: foxtrot golf [ROW:1348] hotel india
1349: foxtrot golf [ROW:1349] hotel india
1350: foxtrot golf [ROW:1350] hotel india
1351: foxtrot golf [ROW:1351] hotel india
1352: foxtrot golf [ROW:1352] hotel india
1353: foxtrot golf [ROW:1353] hotel india
1354: foxtrot golf [ROW:1354] hotel india
1355: foxtrot golf [ROW:1355] hotel india
1356: foxtrot golf [ROW:1356] hotel india
1357: foxtrot golf [ROW:1357] hotel india
1358: foxtrot golf [ROW:1358] hotel india
1359: foxtrot golf [ROW:1359] hotel india
1360: foxtrot golf [ROW:1360] hotel india
1361: foxtrot golf [ROW:1361] hotel india
1362: foxtrot golf [ROW:1362] hotel india
1363: foxtrot golf [ROW:1363] hotel india
1364: foxtrot golf [ROW:1364] hotel india
1365: foxtrot golf [ROW:1365] hotel india
1366: foxtrot golf [ROW:1366] hotel india
1367: foxtrot golf [ROW:1367] hotel india
1368: foxtrot golf [ROW:1368] hotel india
1369: foxtrot golf [ROW:1369] hotel india
1370: foxtrot golf [ROW:1370] hotel india
1371: foxtrot golf [ROW:1371] hotel india
1372: foxtrot golf [ROW:1372] hotel india
1373: foxtrot golf [ROW:1373] hotel india
1374: foxtrot golf [ROW:1374] hotel india
1375: foxtrot golf [ROW:1375] hotel india
1376: foxtrot golf [ROW:1376] hotel india
1377: foxtrot golf [ROW:1377] hotel india
1378: foxtrot golf [ROW:1378] hotel india
1379: foxtrot golf [ROW:1379] hotel india
1380: foxtrot golf [ROW:1380] hotel india
1381: foxtrot golf [ROW:1381] hotel india
1382: foxtrot golf [ROW:1382] hotel india
1383: foxtrot golf [ROW:1383] hotel india
1384: foxtrot golf [ROW:1384] hotel india
1385: foxtrot golf [ROW:1385] hotel india
1386: foxtrot golf [ROW:1386] hotel india
1387: foxtrot golf [ROW:1387] hotel india
1388: foxtrot golf [ROW:1388] hotel india
1389: foxtrot golf [ROW:1389] hotel india
1390: foxtrot golf [ROW:1390] hotel india
1391: foxtrot golf [ROW:1391] hotel india
1392: foxtrot golf [ROW:1392] hotel india
1393: foxtrot golf [ROW:1393] hotel india
1394: foxtrot golf [ROW:1394] hotel india
1395: foxtrot golf [ROW:1395] hotel india
1396: foxtrot golf [ROW:1396] hotel india
1397: foxtrot golf [ROW:1397] hotel india
1398: foxtrot golf [ROW:1398] hotel india
1399: foxtrot golf [ROW:1399] hotel india
1400: foxtrot golf [ROW:1400] hotel india
1401: foxtrot golf [ROW:1401] hotel india
1402: foxtrot golf [ROW:1402] hotel india
1403: foxtrot golf [ROW:1403] hotel india
1404: foxtrot golf [ROW:1404] hotel india
1405: foxtrot golf [ROW:1405] hotel india
1406: foxtrot golf [ROW:1406] hotel india
1407: foxtrot golf [ROW:1407] hotel india
1408: foxtrot golf [ROW:1408] hotel india
1409: foxtrot golf [ROW:1409] hotel india
1410: foxtrot golf [ROW:1410] hotel india
1411: foxtrot golf [ROW:1411] hotel india
1412: foxtrot golf [ROW:1412] hotel india
1413: foxtrot golf [ROW:1413] hotel india
1414: foxtrot golf [ROW:1414] hotel india
1415: foxtrot golf [ROW:1415] hotel india
1416: foxtrot golf [ROW:1416] hotel india
1417: foxtrot golf [ROW:1417] hotel india
1418: foxtrot golf [ROW:1418] hotel india
1419: foxtrot golf [ROW:1419] hotel india
1420: foxtrot golf [ROW:1420] hotel india
1421: foxtrot golf [ROW:1421] hotel india
1422: foxtrot golf [ROW:1422] hotel india
[Output truncated. Continue reading with offset: 1423]
--- RAW OUTPUT END ---

## functions.glob — matches

**Call:** `functions.glob({"pattern":"*.txt","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\large-file-3000-lines.txt
--- RAW OUTPUT END ---

## functions.glob — no matches

**Call:** `functions.glob({"pattern":"*.xyz","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
No files found
--- RAW OUTPUT END ---

## functions.grep — matches

**Call:** `functions.grep({"pattern":"EDIT TARGET","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt"})`

--- RAW OUTPUT START ---
Found 2 matches
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:
  Line 15: --- EDIT TARGET START ---

  Line 17: --- EDIT TARGET END ---
--- RAW OUTPUT END ---

## functions.grep — no matches

**Call:** `functions.grep({"pattern":"NEVER_MATCH_74938","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt"})`

--- RAW OUTPUT START ---
No matches found
--- RAW OUTPUT END ---

## functions.grep — include filter

**Call:** `functions.grep({"pattern":"EDIT TARGET","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts","include":"*.txt"})`

--- RAW OUTPUT START ---
Found 2 matches
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:
  Line 15: --- EDIT TARGET START ---

  Line 17: --- EDIT TARGET END ---
--- RAW OUTPUT END ---

## functions.shell — success

**Call:** `functions.shell({"command":"Write-Output 'CAPTURE_OK'"})`

--- RAW OUTPUT START ---
CAPTURE_OK
--- RAW OUTPUT END ---

## functions.shell — nonzero exit

**Call:** `functions.shell({"command":"Write-Error 'CAPTURE_ERROR'; exit 7"})`

--- RAW OUTPUT START ---
Write-Error 'CAPTURE_ERROR'; exit 7 : CAPTURE_ERROR
    + CategoryInfo          : NotSpecified: (:) [Write-Error], WriteErrorException
    + FullyQualifiedErrorId : Microsoft.PowerShell.Commands.WriteErrorException
 
Exit code: 7
--- RAW OUTPUT END ---

## functions.shell — long output

**Call:** `functions.shell({"command":"1..5000 | ForEach-Object { \"ROW $_\" }"})`

--- RAW OUTPUT START ---
ROW 3001
ROW 3002
ROW 3003
ROW 3004
ROW 3005
ROW 3006
ROW 3007
ROW 3008
ROW 3009
ROW 3010
ROW 3011
ROW 3012
ROW 3013
ROW 3014
ROW 3015
ROW 3016
ROW 3017
ROW 3018
ROW 3019
ROW 3020
ROW 3021
ROW 3022
ROW 3023
ROW 3024
ROW 3025
ROW 3026
ROW 3027
ROW 3028
ROW 3029
ROW 3030
ROW 3031
ROW 3032
ROW 3033
ROW 3034
ROW 3035
ROW 3036
ROW 3037
ROW 3038
ROW 3039
ROW 3040
ROW 3041
ROW 3042
ROW 3043
ROW 3044
ROW 3045
ROW 3046
ROW 3047
ROW 3048
ROW 3049
ROW 3050
ROW 3051
ROW 3052
ROW 3053
ROW 3054
ROW 3055
ROW 3056
ROW 3057
ROW 3058
ROW 3059
ROW 3060
ROW 3061
ROW 3062
ROW 3063
ROW 3064
ROW 3065
ROW 3066
ROW 3067
ROW 3068
ROW 3069
ROW 3070
ROW 3071
ROW 3072
ROW 3073
ROW 3074
ROW 3075
ROW 3076
ROW 3077
ROW 3078
ROW 3079
ROW 3080
ROW 3081
ROW 3082
ROW 3083
ROW 3084
ROW 3085
ROW 3086
ROW 3087
ROW 3088
ROW 3089
ROW 3090
ROW 3091
ROW 3092
ROW 3093
ROW 3094
ROW 3095
ROW 3096
ROW 3097
ROW 3098
ROW 3099
ROW 3100
ROW 3101
ROW 3102
ROW 3103
ROW 3104
ROW 3105
ROW 3106
ROW 3107
ROW 3108
ROW 3109
ROW 3110
ROW 3111
ROW 3112
ROW 3113
ROW 3114
ROW 3115
ROW 3116
ROW 3117
ROW 3118
ROW 3119
ROW 3120
ROW 3121
ROW 3122
ROW 3123
ROW 3124
ROW 3125
ROW 3126
ROW 3127
ROW 3128
ROW 3129
ROW 3130
ROW 3131
ROW 3132
ROW 3133
ROW 3134
ROW 3135
ROW 3136
ROW 3137
ROW 3138
ROW 3139
ROW 3140
ROW 3141
ROW 3142
ROW 3143
ROW 3144
ROW 3145
ROW 3146
ROW 3147
ROW 3148
ROW 3149
ROW 3150
ROW 3151
ROW 3152
ROW 3153
ROW 3154
ROW 3155
ROW 3156
ROW 3157
ROW 3158
ROW 3159
ROW 3160
ROW 3161
ROW 3162
ROW 3163
ROW 3164
ROW 3165
ROW 3166
ROW 3167
ROW 3168
ROW 3169
ROW 3170
ROW 3171
ROW 3172
ROW 3173
ROW 3174
ROW 3175
ROW 3176
ROW 3177
ROW 3178
ROW 3179
ROW 3180
ROW 3181
ROW 3182
ROW 3183
ROW 3184
ROW 3185
ROW 3186
ROW 3187
ROW 3188
ROW 3189
ROW 3190
ROW 3191
ROW 3192
ROW 3193
ROW 3194
ROW 3195
ROW 3196
ROW 3197
ROW 3198
ROW 3199
ROW 3200
ROW 3201
ROW 3202
ROW 3203
ROW 3204
ROW 3205
ROW 3206
ROW 3207
ROW 3208
ROW 3209
ROW 3210
ROW 3211
ROW 3212
ROW 3213
ROW 3214
ROW 3215
ROW 3216
ROW 3217
ROW 3218
ROW 3219
ROW 3220
ROW 3221
ROW 3222
ROW 3223
ROW 3224
ROW 3225
ROW 3226
ROW 3227
ROW 3228
ROW 3229
ROW 3230
ROW 3231
ROW 3232
ROW 3233
ROW 3234
ROW 3235
ROW 3236
ROW 3237
ROW 3238
ROW 3239
ROW 3240
ROW 3241
ROW 3242
ROW 3243
ROW 3244
ROW 3245
ROW 3246
ROW 3247
ROW 3248
ROW 3249
ROW 3250
ROW 3251
ROW 3252
ROW 3253
ROW 3254
ROW 3255
ROW 3256
ROW 3257
ROW 3258
ROW 3259
ROW 3260
ROW 3261
ROW 3262
ROW 3263
ROW 3264
ROW 3265
ROW 3266
ROW 3267
ROW 3268
ROW 3269
ROW 3270
ROW 3271
ROW 3272
ROW 3273
ROW 3274
ROW 3275
ROW 3276
ROW 3277
ROW 3278
ROW 3279
ROW 3280
ROW 3281
ROW 3282
ROW 3283
ROW 3284
ROW 3285
ROW 3286
ROW 3287
ROW 3288
ROW 3289
ROW 3290
ROW 3291
ROW 3292
ROW 3293
ROW 3294
ROW 3295
ROW 3296
ROW 3297
ROW 3298
ROW 3299
ROW 3300
ROW 3301
ROW 3302
ROW 3303
ROW 3304
ROW 3305
ROW 3306
ROW 3307
ROW 3308
ROW 3309
ROW 3310
ROW 3311
ROW 3312
ROW 3313
ROW 3314
ROW 3315
ROW 3316
ROW 3317
ROW 3318
ROW 3319
ROW 3320
ROW 3321
ROW 3322
ROW 3323
ROW 3324
ROW 3325
ROW 3326
ROW 3327
ROW 3328
ROW 3329
ROW 3330
ROW 3331
ROW 3332
ROW 3333
ROW 3334
ROW 3335
ROW 3336
ROW 3337
ROW 3338
ROW 3339
ROW 3340
ROW 3341
ROW 3342
ROW 3343
ROW 3344
ROW 3345
ROW 3346
ROW 3347
ROW 3348
ROW 3349
ROW 3350
ROW 3351
ROW 3352
ROW 3353
ROW 3354
ROW 3355
ROW 3356
ROW 3357
ROW 3358
ROW 3359
ROW 3360
ROW 3361
ROW 3362
ROW 3363
ROW 3364
ROW 3365
ROW 3366
ROW 3367
ROW 3368
ROW 3369
ROW 3370
ROW 3371
ROW 3372
ROW 3373
ROW 3374
ROW 3375
ROW 3376
ROW 3377
ROW 3378
ROW 3379
ROW 3380
ROW 3381
ROW 3382
ROW 3383
ROW 3384
ROW 3385
ROW 3386
ROW 3387
ROW 3388
ROW 3389
ROW 3390
ROW 3391
ROW 3392
ROW 3393
ROW 3394
ROW 3395
ROW 3396
ROW 3397
ROW 3398
ROW 3399
ROW 3400
ROW 3401
ROW 3402
ROW 3403
ROW 3404
ROW 3405
ROW 3406
ROW 3407
ROW 3408
ROW 3409
ROW 3410
ROW 3411
ROW 3412
ROW 3413
ROW 3414
ROW 3415
ROW 3416
ROW 3417
ROW 3418
ROW 3419
ROW 3420
ROW 3421
ROW 3422
ROW 3423
ROW 3424
ROW 3425
ROW 3426
ROW 3427
ROW 3428
ROW 3429
ROW 3430
ROW 3431
ROW 3432
ROW 3433
ROW 3434
ROW 3435
ROW 3436
ROW 3437
ROW 3438
ROW 3439
ROW 3440
ROW 3441
ROW 3442
ROW 3443
ROW 3444
ROW 3445
ROW 3446
ROW 3447
ROW 3448
ROW 3449
ROW 3450
ROW 3451
ROW 3452
ROW 3453
ROW 3454
ROW 3455
ROW 3456
ROW 3457
ROW 3458
ROW 3459
ROW 3460
ROW 3461
ROW 3462
ROW 3463
ROW 3464
ROW 3465
ROW 3466
ROW 3467
ROW 3468
ROW 3469
ROW 3470
ROW 3471
ROW 3472
ROW 3473
ROW 3474
ROW 3475
ROW 3476
ROW 3477
ROW 3478
ROW 3479
ROW 3480
ROW 3481
ROW 3482
ROW 3483
ROW 3484
ROW 3485
ROW 3486
ROW 3487
ROW 3488
ROW 3489
ROW 3490
ROW 3491
ROW 3492
ROW 3493
ROW 3494
ROW 3495
ROW 3496
ROW 3497
ROW 3498
ROW 3499
ROW 3500
ROW 3501
ROW 3502
ROW 3503
ROW 3504
ROW 3505
ROW 3506
ROW 3507
ROW 3508
ROW 3509
ROW 3510
ROW 3511
ROW 3512
ROW 3513
ROW 3514
ROW 3515
ROW 3516
ROW 3517
ROW 3518
ROW 3519
ROW 3520
ROW 3521
ROW 3522
ROW 3523
ROW 3524
ROW 3525
ROW 3526
ROW 3527
ROW 3528
ROW 3529
ROW 3530
ROW 3531
ROW 3532
ROW 3533
ROW 3534
ROW 3535
ROW 3536
ROW 3537
ROW 3538
ROW 3539
ROW 3540
ROW 3541
ROW 3542
ROW 3543
ROW 3544
ROW 3545
ROW 3546
ROW 3547
ROW 3548
ROW 3549
ROW 3550
ROW 3551
ROW 3552
ROW 3553
ROW 3554
ROW 3555
ROW 3556
ROW 3557
ROW 3558
ROW 3559
ROW 3560
ROW 3561
ROW 3562
ROW 3563
ROW 3564
ROW 3565
ROW 3566
ROW 3567
ROW 3568
ROW 3569
ROW 3570
ROW 3571
ROW 3572
ROW 3573
ROW 3574
ROW 3575
ROW 3576
ROW 3577
ROW 3578
ROW 3579
ROW 3580
ROW 3581
ROW 3582
ROW 3583
ROW 3584
ROW 3585
ROW 3586
ROW 3587
ROW 3588
ROW 3589
ROW 3590
ROW 3591
ROW 3592
ROW 3593
ROW 3594
ROW 3595
ROW 3596
ROW 3597
ROW 3598
ROW 3599
ROW 3600
ROW 3601
ROW 3602
ROW 3603
ROW 3604
ROW 3605
ROW 3606
ROW 3607
ROW 3608
ROW 3609
ROW 3610
ROW 3611
ROW 3612
ROW 3613
ROW 3614
ROW 3615
ROW 3616
ROW 3617
ROW 3618
ROW 3619
ROW 3620
ROW 3621
ROW 3622
ROW 3623
ROW 3624
ROW 3625
ROW 3626
ROW 3627
ROW 3628
ROW 3629
ROW 3630
ROW 3631
ROW 3632
ROW 3633
ROW 3634
ROW 3635
ROW 3636
ROW 3637
ROW 3638
ROW 3639
ROW 3640
ROW 3641
ROW 3642
ROW 3643
ROW 3644
ROW 3645
ROW 3646
ROW 3647
ROW 3648
ROW 3649
ROW 3650
ROW 3651
ROW 3652
ROW 3653
ROW 3654
ROW 3655
ROW 3656
ROW 3657
ROW 3658
ROW 3659
ROW 3660
ROW 3661
ROW 3662
ROW 3663
ROW 3664
ROW 3665
ROW 3666
ROW 3667
ROW 3668
ROW 3669
ROW 3670
ROW 3671
ROW 3672
ROW 3673
ROW 3674
ROW 3675
ROW 3676
ROW 3677
ROW 3678
ROW 3679
ROW 3680
ROW 3681
ROW 3682
ROW 3683
ROW 3684
ROW 3685
ROW 3686
ROW 3687
ROW 3688
ROW 3689
ROW 3690
ROW 3691
ROW 3692
ROW 3693
ROW 3694
ROW 3695
ROW 3696
ROW 3697
ROW 3698
ROW 3699
ROW 3700
ROW 3701
ROW 3702
ROW 3703
ROW 3704
ROW 3705
ROW 3706
ROW 3707
ROW 3708
ROW 3709
ROW 3710
ROW 3711
ROW 3712
ROW 3713
ROW 3714
ROW 3715
ROW 3716
ROW 3717
ROW 3718
ROW 3719
ROW 3720
ROW 3721
ROW 3722
ROW 3723
ROW 3724
ROW 3725
ROW 3726
ROW 3727
ROW 3728
ROW 3729
ROW 3730
ROW 3731
ROW 3732
ROW 3733
ROW 3734
ROW 3735
ROW 3736
ROW 3737
ROW 3738
ROW 3739
ROW 3740
ROW 3741
ROW 3742
ROW 3743
ROW 3744
ROW 3745
ROW 3746
ROW 3747
ROW 3748
ROW 3749
ROW 3750
ROW 3751
ROW 3752
ROW 3753
ROW 3754
ROW 3755
ROW 3756
ROW 3757
ROW 3758
ROW 3759
ROW 3760
ROW 3761
ROW 3762
ROW 3763
ROW 3764
ROW 3765
ROW 3766
ROW 3767
ROW 3768
ROW 3769
ROW 3770
ROW 3771
ROW 3772
ROW 3773
ROW 3774
ROW 3775
ROW 3776
ROW 3777
ROW 3778
ROW 3779
ROW 3780
ROW 3781
ROW 3782
ROW 3783
ROW 3784
ROW 3785
ROW 3786
ROW 3787
ROW 3788
ROW 3789
ROW 3790
ROW 3791
ROW 3792
ROW 3793
ROW 3794
ROW 3795
ROW 3796
ROW 3797
ROW 3798
ROW 3799
ROW 3800
ROW 3801
ROW 3802
ROW 3803
ROW 3804
ROW 3805
ROW 3806
ROW 3807
ROW 3808
ROW 3809
ROW 3810
ROW 3811
ROW 3812
ROW 3813
ROW 3814
ROW 3815
ROW 3816
ROW 3817
ROW 3818
ROW 3819
ROW 3820
ROW 3821
ROW 3822
ROW 3823
ROW 3824
ROW 3825
ROW 3826
ROW 3827
ROW 3828
ROW 3829
ROW 3830
ROW 3831
ROW 3832
ROW 3833
ROW 3834
ROW 3835
ROW 3836
ROW 3837
ROW 3838
ROW 3839
ROW 3840
ROW 3841
ROW 3842
ROW 3843
ROW 3844
ROW 3845
ROW 3846
ROW 3847
ROW 3848
ROW 3849
ROW 3850
ROW 3851
ROW 3852
ROW 3853
ROW 3854
ROW 3855
ROW 3856
ROW 3857
ROW 3858
ROW 3859
ROW 3860
ROW 3861
ROW 3862
ROW 3863
ROW 3864
ROW 3865
ROW 3866
ROW 3867
ROW 3868
ROW 3869
ROW 3870
ROW 3871
ROW 3872
ROW 3873
ROW 3874
ROW 3875
ROW 3876
ROW 3877
ROW 3878
ROW 3879
ROW 3880
ROW 3881
ROW 3882
ROW 3883
ROW 3884
ROW 3885
ROW 3886
ROW 3887
ROW 3888
ROW 3889
ROW 3890
ROW 3891
ROW 3892
ROW 3893
ROW 3894
ROW 3895
ROW 3896
ROW 3897
ROW 3898
ROW 3899
ROW 3900
ROW 3901
ROW 3902
ROW 3903
ROW 3904
ROW 3905
ROW 3906
ROW 3907
ROW 3908
ROW 3909
ROW 3910
ROW 3911
ROW 3912
ROW 3913
ROW 3914
ROW 3915
ROW 3916
ROW 3917
ROW 3918
ROW 3919
ROW 3920
ROW 3921
ROW 3922
ROW 3923
ROW 3924
ROW 3925
ROW 3926
ROW 3927
ROW 3928
ROW 3929
ROW 3930
ROW 3931
ROW 3932
ROW 3933
ROW 3934
ROW 3935
ROW 3936
ROW 3937
ROW 3938
ROW 3939
ROW 3940
ROW 3941
ROW 3942
ROW 3943
ROW 3944
ROW 3945
ROW 3946
ROW 3947
ROW 3948
ROW 3949
ROW 3950
ROW 3951
ROW 3952
ROW 3953
ROW 3954
ROW 3955
ROW 3956
ROW 3957
ROW 3958
ROW 3959
ROW 3960
ROW 3961
ROW 3962
ROW 3963
ROW 3964
ROW 3965
ROW 3966
ROW 3967
ROW 3968
ROW 3969
ROW 3970
ROW 3971
ROW 3972
ROW 3973
ROW 3974
ROW 3975
ROW 3976
ROW 3977
ROW 3978
ROW 3979
ROW 3980
ROW 3981
ROW 3982
ROW 3983
ROW 3984
ROW 3985
ROW 3986
ROW 3987
ROW 3988
ROW 3989
ROW 3990
ROW 3991
ROW 3992
ROW 3993
ROW 3994
ROW 3995
ROW 3996
ROW 3997
ROW 3998
ROW 3999
ROW 4000
ROW 4001
ROW 4002
ROW 4003
ROW 4004
ROW 4005
ROW 4006
ROW 4007
ROW 4008
ROW 4009
ROW 4010
ROW 4011
ROW 4012
ROW 4013
ROW 4014
ROW 4015
ROW 4016
ROW 4017
ROW 4018
ROW 4019
ROW 4020
ROW 4021
ROW 4022
ROW 4023
ROW 4024
ROW 4025
ROW 4026
ROW 4027
ROW 4028
ROW 4029
ROW 4030
ROW 4031
ROW 4032
ROW 4033
ROW 4034
ROW 4035
ROW 4036
ROW 4037
ROW 4038
ROW 4039
ROW 4040
ROW 4041
ROW 4042
ROW 4043
ROW 4044
ROW 4045
ROW 4046
ROW 4047
ROW 4048
ROW 4049
ROW 4050
ROW 4051
ROW 4052
ROW 4053
ROW 4054
ROW 4055
ROW 4056
ROW 4057
ROW 4058
ROW 4059
ROW 4060
ROW 4061
ROW 4062
ROW 4063
ROW 4064
ROW 4065
ROW 4066
ROW 4067
ROW 4068
ROW 4069
ROW 4070
ROW 4071
ROW 4072
ROW 4073
ROW 4074
ROW 4075
ROW 4076
ROW 4077
ROW 4078
ROW 4079
ROW 4080
ROW 4081
ROW 4082
ROW 4083
ROW 4084
ROW 4085
ROW 4086
ROW 4087
ROW 4088
ROW 4089
ROW 4090
ROW 4091
ROW 4092
ROW 4093
ROW 4094
ROW 4095
ROW 4096
ROW 4097
ROW 4098
ROW 4099
ROW 4100
ROW 4101
ROW 4102
ROW 4103
ROW 4104
ROW 4105
ROW 4106
ROW 4107
ROW 4108
ROW 4109
ROW 4110
ROW 4111
ROW 4112
ROW 4113
ROW 4114
ROW 4115
ROW 4116
ROW 4117
ROW 4118
ROW 4119
ROW 4120
ROW 4121
ROW 4122
ROW 4123
ROW 4124
ROW 4125
ROW 4126
ROW 4127
ROW 4128
ROW 4129
ROW 4130
ROW 4131
ROW 4132
ROW 4133
ROW 4134
ROW 4135
ROW 4136
ROW 4137
ROW 4138
ROW 4139
ROW 4140
ROW 4141
ROW 4142
ROW 4143
ROW 4144
ROW 4145
ROW 4146
ROW 4147
ROW 4148
ROW 4149
ROW 4150
ROW 4151
ROW 4152
ROW 4153
ROW 4154
ROW 4155
ROW 4156
ROW 4157
ROW 4158
ROW 4159
ROW 4160
ROW 4161
ROW 4162
ROW 4163
ROW 4164
ROW 4165
ROW 4166
ROW 4167
ROW 4168
ROW 4169
ROW 4170
ROW 4171
ROW 4172
ROW 4173
ROW 4174
ROW 4175
ROW 4176
ROW 4177
ROW 4178
ROW 4179
ROW 4180
ROW 4181
ROW 4182
ROW 4183
ROW 4184
ROW 4185
ROW 4186
ROW 4187
ROW 4188
ROW 4189
ROW 4190
ROW 4191
ROW 4192
ROW 4193
ROW 4194
ROW 4195
ROW 4196
ROW 4197
ROW 4198
ROW 4199
ROW 4200
ROW 4201
ROW 4202
ROW 4203
ROW 4204
ROW 4205
ROW 4206
ROW 4207
ROW 4208
ROW 4209
ROW 4210
ROW 4211
ROW 4212
ROW 4213
ROW 4214
ROW 4215
ROW 4216
ROW 4217
ROW 4218
ROW 4219
ROW 4220
ROW 4221
ROW 4222
ROW 4223
ROW 4224
ROW 4225
ROW 4226
ROW 4227
ROW 4228
ROW 4229
ROW 4230
ROW 4231
ROW 4232
ROW 4233
ROW 4234
ROW 4235
ROW 4236
ROW 4237
ROW 4238
ROW 4239
ROW 4240
ROW 4241
ROW 4242
ROW 4243
ROW 4244
ROW 4245
ROW 4246
ROW 4247
ROW 4248
ROW 4249
ROW 4250
ROW 4251
ROW 4252
ROW 4253
ROW 4254
ROW 4255
ROW 4256
ROW 4257
ROW 4258
ROW 4259
ROW 4260
ROW 4261
ROW 4262
ROW 4263
ROW 4264
ROW 4265
ROW 4266
ROW 4267
ROW 4268
ROW 4269
ROW 4270
ROW 4271
ROW 4272
ROW 4273
ROW 4274
ROW 4275
ROW 4276
ROW 4277
ROW 4278
ROW 4279
ROW 4280
ROW 4281
ROW 4282
ROW 4283
ROW 4284
ROW 4285
ROW 4286
ROW 4287
ROW 4288
ROW 4289
ROW 4290
ROW 4291
ROW 4292
ROW 4293
ROW 4294
ROW 4295
ROW 4296
ROW 4297
ROW 4298
ROW 4299
ROW 4300
ROW 4301
ROW 4302
ROW 4303
ROW 4304
ROW 4305
ROW 4306
ROW 4307
ROW 4308
ROW 4309
ROW 4310
ROW 4311
ROW 4312
ROW 4313
ROW 4314
ROW 4315
ROW 4316
ROW 4317
ROW 4318
ROW 4319
ROW 4320
ROW 4321
ROW 4322
ROW 4323
ROW 4324
ROW 4325
ROW 4326
ROW 4327
ROW 4328
ROW 4329
ROW 4330
ROW 4331
ROW 4332
ROW 4333
ROW 4334
ROW 4335
ROW 4336
ROW 4337
ROW 4338
ROW 4339
ROW 4340
ROW 4341
ROW 4342
ROW 4343
ROW 4344
ROW 4345
ROW 4346
ROW 4347
ROW 4348
ROW 4349
ROW 4350
ROW 4351
ROW 4352
ROW 4353
ROW 4354
ROW 4355
ROW 4356
ROW 4357
ROW 4358
ROW 4359
ROW 4360
ROW 4361
ROW 4362
ROW 4363
ROW 4364
ROW 4365
ROW 4366
ROW 4367
ROW 4368
ROW 4369
ROW 4370
ROW 4371
ROW 4372
ROW 4373
ROW 4374
ROW 4375
ROW 4376
ROW 4377
ROW 4378
ROW 4379
ROW 4380
ROW 4381
ROW 4382
ROW 4383
ROW 4384
ROW 4385
ROW 4386
ROW 4387
ROW 4388
ROW 4389
ROW 4390
ROW 4391
ROW 4392
ROW 4393
ROW 4394
ROW 4395
ROW 4396
ROW 4397
ROW 4398
ROW 4399
ROW 4400
ROW 4401
ROW 4402
ROW 4403
ROW 4404
ROW 4405
ROW 4406
ROW 4407
ROW 4408
ROW 4409
ROW 4410
ROW 4411
ROW 4412
ROW 4413
ROW 4414
ROW 4415
ROW 4416
ROW 4417
ROW 4418
ROW 4419
ROW 4420
ROW 4421
ROW 4422
ROW 4423
ROW 4424
ROW 4425
ROW 4426
ROW 4427
ROW 4428
ROW 4429
ROW 4430
ROW 4431
ROW 4432
ROW 4433
ROW 4434
ROW 4435
ROW 4436
ROW 4437
ROW 4438
ROW 4439
ROW 4440
ROW 4441
ROW 4442
ROW 4443
ROW 4444
ROW 4445
ROW 4446
ROW 4447
ROW 4448
ROW 4449
ROW 4450
ROW 4451
ROW 4452
ROW 4453
ROW 4454
ROW 4455
ROW 4456
ROW 4457
ROW 4458
ROW 4459
ROW 4460
ROW 4461
ROW 4462
ROW 4463
ROW 4464
ROW 4465
ROW 4466
ROW 4467
ROW 4468
ROW 4469
ROW 4470
ROW 4471
ROW 4472
ROW 4473
ROW 4474
ROW 4475
ROW 4476
ROW 4477
ROW 4478
ROW 4479
ROW 4480
ROW 4481
ROW 4482
ROW 4483
ROW 4484
ROW 4485
ROW 4486
ROW 4487
ROW 4488
ROW 4489
ROW 4490
ROW 4491
ROW 4492
ROW 4493
ROW 4494
ROW 4495
ROW 4496
ROW 4497
ROW 4498
ROW 4499
ROW 4500
ROW 4501
ROW 4502
ROW 4503
ROW 4504
ROW 4505
ROW 4506
ROW 4507
ROW 4508
ROW 4509
ROW 4510
ROW 4511
ROW 4512
ROW 4513
ROW 4514
ROW 4515
ROW 4516
ROW 4517
ROW 4518
ROW 4519
ROW 4520
ROW 4521
ROW 4522
ROW 4523
ROW 4524
ROW 4525
ROW 4526
ROW 4527
ROW 4528
ROW 4529
ROW 4530
ROW 4531
ROW 4532
ROW 4533
ROW 4534
ROW 4535
ROW 4536
ROW 4537
ROW 4538
ROW 4539
ROW 4540
ROW 4541
ROW 4542
ROW 4543
ROW 4544
ROW 4545
ROW 4546
ROW 4547
ROW 4548
ROW 4549
ROW 4550
ROW 4551
ROW 4552
ROW 4553
ROW 4554
ROW 4555
ROW 4556
ROW 4557
ROW 4558
ROW 4559
ROW 4560
ROW 4561
ROW 4562
ROW 4563
ROW 4564
ROW 4565
ROW 4566
ROW 4567
ROW 4568
ROW 4569
ROW 4570
ROW 4571
ROW 4572
ROW 4573
ROW 4574
ROW 4575
ROW 4576
ROW 4577
ROW 4578
ROW 4579
ROW 4580
ROW 4581
ROW 4582
ROW 4583
ROW 4584
ROW 4585
ROW 4586
ROW 4587
ROW 4588
ROW 4589
ROW 4590
ROW 4591
ROW 4592
ROW 4593
ROW 4594
ROW 4595
ROW 4596
ROW 4597
ROW 4598
ROW 4599
ROW 4600
ROW 4601
ROW 4602
ROW 4603
ROW 4604
ROW 4605
ROW 4606
ROW 4607
ROW 4608
ROW 4609
ROW 4610
ROW 4611
ROW 4612
ROW 4613
ROW 4614
ROW 4615
ROW 4616
ROW 4617
ROW 4618
ROW 4619
ROW 4620
ROW 4621
ROW 4622
ROW 4623
ROW 4624
ROW 4625
ROW 4626
ROW 4627
ROW 4628
ROW 4629
ROW 4630
ROW 4631
ROW 4632
ROW 4633
ROW 4634
ROW 4635
ROW 4636
ROW 4637
ROW 4638
ROW 4639
ROW 4640
ROW 4641
ROW 4642
ROW 4643
ROW 4644
ROW 4645
ROW 4646
ROW 4647
ROW 4648
ROW 4649
ROW 4650
ROW 4651
ROW 4652
ROW 4653
ROW 4654
ROW 4655
ROW 4656
ROW 4657
ROW 4658
ROW 4659
ROW 4660
ROW 4661
ROW 4662
ROW 4663
ROW 4664
ROW 4665
ROW 4666
ROW 4667
ROW 4668
ROW 4669
ROW 4670
ROW 4671
ROW 4672
ROW 4673
ROW 4674
ROW 4675
ROW 4676
ROW 4677
ROW 4678
ROW 4679
ROW 4680
ROW 4681
ROW 4682
ROW 4683
ROW 4684
ROW 4685
ROW 4686
ROW 4687
ROW 4688
ROW 4689
ROW 4690
ROW 4691
ROW 4692
ROW 4693
ROW 4694
ROW 4695
ROW 4696
ROW 4697
ROW 4698
ROW 4699
ROW 4700
ROW 4701
ROW 4702
ROW 4703
ROW 4704
ROW 4705
ROW 4706
ROW 4707
ROW 4708
ROW 4709
ROW 4710
ROW 4711
ROW 4712
ROW 4713
ROW 4714
ROW 4715
ROW 4716
ROW 4717
ROW 4718
ROW 4719
ROW 4720
ROW 4721
ROW 4722
ROW 4723
ROW 4724
ROW 4725
ROW 4726
ROW 4727
ROW 4728
ROW 4729
ROW 4730
ROW 4731
ROW 4732
ROW 4733
ROW 4734
ROW 4735
ROW 4736
ROW 4737
ROW 4738
ROW 4739
ROW 4740
ROW 4741
ROW 4742
ROW 4743
ROW 4744
ROW 4745
ROW 4746
ROW 4747
ROW 4748
ROW 4749
ROW 4750
ROW 4751
ROW 4752
ROW 4753
ROW 4754
ROW 4755
ROW 4756
ROW 4757
ROW 4758
ROW 4759
ROW 4760
ROW 4761
ROW 4762
ROW 4763
ROW 4764
ROW 4765
ROW 4766
ROW 4767
ROW 4768
ROW 4769
ROW 4770
ROW 4771
ROW 4772
ROW 4773
ROW 4774
ROW 4775
ROW 4776
ROW 4777
ROW 4778
ROW 4779
ROW 4780
ROW 4781
ROW 4782
ROW 4783
ROW 4784
ROW 4785
ROW 4786
ROW 4787
ROW 4788
ROW 4789
ROW 4790
ROW 4791
ROW 4792
ROW 4793
ROW 4794
ROW 4795
ROW 4796
ROW 4797
ROW 4798
ROW 4799
ROW 4800
ROW 4801
ROW 4802
ROW 4803
ROW 4804
ROW 4805
ROW 4806
ROW 4807
ROW 4808
ROW 4809
ROW 4810
ROW 4811
ROW 4812
ROW 4813
ROW 4814
ROW 4815
ROW 4816
ROW 4817
ROW 4818
ROW 4819
ROW 4820
ROW 4821
ROW 4822
ROW 4823
ROW 4824
ROW 4825
ROW 4826
ROW 4827
ROW 4828
ROW 4829
ROW 4830
ROW 4831
ROW 4832
ROW 4833
ROW 4834
ROW 4835
ROW 4836
ROW 4837
ROW 4838
ROW 4839
ROW 4840
ROW 4841
ROW 4842
ROW 4843
ROW 4844
ROW 4845
ROW 4846
ROW 4847
ROW 4848
ROW 4849
ROW 4850
ROW 4851
ROW 4852
ROW 4853
ROW 4854
ROW 4855
ROW 4856
ROW 4857
ROW 4858
ROW 4859
ROW 4860
ROW 4861
ROW 4862
ROW 4863
ROW 4864
ROW 4865
ROW 4866
ROW 4867
ROW 4868
ROW 4869
ROW 4870
ROW 4871
ROW 4872
ROW 4873
ROW 4874
ROW 4875
ROW 4876
ROW 4877
ROW 4878
ROW 4879
ROW 4880
ROW 4881
ROW 4882
ROW 4883
ROW 4884
ROW 4885
ROW 4886
ROW 4887
ROW 4888
ROW 4889
ROW 4890
ROW 4891
ROW 4892
ROW 4893
ROW 4894
ROW 4895
ROW 4896
ROW 4897
ROW 4898
ROW 4899
ROW 4900
ROW 4901
ROW 4902
ROW 4903
ROW 4904
ROW 4905
ROW 4906
ROW 4907
ROW 4908
ROW 4909
ROW 4910
ROW 4911
ROW 4912
ROW 4913
ROW 4914
ROW 4915
ROW 4916
ROW 4917
ROW 4918
ROW 4919
ROW 4920
ROW 4921
ROW 4922
ROW 4923
ROW 4924
ROW 4925
ROW 4926
ROW 4927
ROW 4928
ROW 4929
ROW 4930
ROW 4931
ROW 4932
ROW 4933
ROW 4934
ROW 4935
ROW 4936
ROW 4937
ROW 4938
ROW 4939
ROW 4940
ROW 4941
ROW 4942
ROW 4943
ROW 4944
ROW 4945
ROW 4946
ROW 4947
ROW 4948
ROW 4949
ROW 4950
ROW 4951
ROW 4952
ROW 4953
ROW 4954
ROW 4955
ROW 4956
ROW 4957
ROW 4958
ROW 4959
ROW 4960
ROW 4961
ROW 4962
ROW 4963
ROW 4964
ROW 4965
ROW 4966
ROW 4967
ROW 4968
ROW 4969
ROW 4970
ROW 4971
ROW 4972
ROW 4973
ROW 4974
ROW 4975
ROW 4976
ROW 4977
ROW 4978
ROW 4979
ROW 4980
ROW 4981
ROW 4982
ROW 4983
ROW 4984
ROW 4985
ROW 4986
ROW 4987
ROW 4988
ROW 4989
ROW 4990
ROW 4991
ROW 4992
ROW 4993
ROW 4994
ROW 4995
ROW 4996
ROW 4997
ROW 4998
ROW 4999
ROW 5000

[showing lines 3001-5000 of 5000; full output saved to C:\Users\tgurt\.local\share\opencode\shell\b27e9ef462535a30ae764504d83aa0d9670e945f\sh_1f021c072002NA9y1k7IDEUNus.out]
--- RAW OUTPUT END ---

## functions.question — answered test question

**Call:** `functions.question({"questions":[{"question":"Capture test: which option should I record for this tool-output probe?","header":"Capture probe","options":[{"label":"Option A (Recommended)","description":"Record the first option."},{"label":"Option B","description":"Record the second option."}]}]})`

--- RAW OUTPUT START ---
User has answered your questions: "Capture test: which option should I record for this tool-output probe?"="Option A (Recommended)". You can now continue with the user's answers in mind.
--- RAW OUTPUT END ---

## functions.subagent — spawn test

**Call:** `functions.subagent({"agent":"general","description":"Test spawn output format","prompt":"Tool-output format probe only. Reply with the single word READY. Do not read or edit any files."})`

--- RAW OUTPUT START ---
The subagent is working in the background (sessionID: ses_e0fdddc20ffetS3OM18QXOM3jF). You will receive a [subagent] message when it goes idle, and again after each later round of work.
DO NOT sleep, poll, proactively check on its progress, or duplicate this subagent's work.
Work on non-overlapping tasks, or briefly tell the user what you launched and end your response.
To send it follow-ups or change its instructions while it runs, call subagent again with this sessionID.
--- RAW OUTPUT END ---

## functions.subagent — completion notification

**Call:** `completion of functions.subagent({"agent":"general","description":"Test spawn output format","prompt":"Tool-output format probe only. Reply with the single word READY. Do not read or edit any files."})`

--- RAW OUTPUT START ---
[subagent sessionID="ses_e0fdddc20ffetS3OM18QXOM3jF" state="completed" description="Test spawn output format"]
READY
[/subagent]
--- RAW OUTPUT END ---

## functions.opencode_worktree_remove — nonexistent worktree error

**Call:** `functions.opencode_worktree_remove({"directory":"C:\\AI\\MOSAIC\\MOSAIC\\NONEXISTENT_CAPTURE_WORKTREE"})`

--- RAW OUTPUT START ---
{"error":{"type":"tool.execution","message":"Unable to remove worktree C:\\AI\\MOSAIC\\MOSAIC\\NONEXISTENT_CAPTURE_WORKTREE: the directory does not exist or is not a directory."},"content":[]}
--- RAW OUTPUT END ---

## functions.opencode_worktree_remove — Not captured: success would remove a real worktree and is destructive

## functions.skill — nonexistent skill error

**Call:** `functions.skill({"id":"nonexistent-capture-skill-2026"})`

--- RAW OUTPUT START ---
{"error":{"type":"tool.execution","message":"Unable to load skill nonexistent-capture-skill-2026"},"content":[]}
--- RAW OUTPUT END ---

## functions.skill — Not captured: no matching skill known in this context

## functions.patch — successful add of raw capture

**Call:** `functions.patch("*** Add File: HarnessKnowledge/SystemPromptCapture/OpenCodeV2/v2.3.2/PrimaryAgent/SystemPrompt-raw.md ...")`

--- RAW OUTPUT START ---
Success. Updated the following files:
A HarnessKnowledge/SystemPromptCapture/OpenCodeV2/v2.3.2/PrimaryAgent/SystemPrompt-raw.md
--- RAW OUTPUT END ---

## functions.patch — nonexistent file error

**Call:** `functions.patch("*** Update File: HarnessKnowledge/SystemPromptCapture/OpenCodeV2/v2.3.2/PrimaryAgent/does-not-exist-capture-probe.txt ...")`

--- RAW OUTPUT START ---
{"error":{"type":"tool.execution","message":"patch verification failed: Failed to read file to update C:\\AI\\MOSAIC\\MOSAIC\\HarnessKnowledge\\SystemPromptCapture\\OpenCodeV2\\v2.3.2\\PrimaryAgent\\does-not-exist-capture-probe.txt: file does not exist"},"content":[]}
--- RAW OUTPUT END ---

## multi_tool_use.parallel — Not captured: wrapper emitted no separate result; individual child results are recorded in their own sections above

## edit — Not captured: no standalone edit tool is exposed in this session

## write — Not captured: no standalone write tool is exposed in this session
