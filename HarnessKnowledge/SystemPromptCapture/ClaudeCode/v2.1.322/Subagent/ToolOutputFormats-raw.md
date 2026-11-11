# Tool Output Formats — Raw Captures

| Field | Value |
|-------|-------|
| **Harness** | Claude Code |
| **Harness version** | v2.1.322 |
| **Context** | Subagent session |
| **Captured** | 2026-11-11 |
| **Formatting** | XML tags use bracket notation: `<tag>` -> `[tag]` to avoid rendering conflicts |

---

## Wrapper — Successful tool result

**Call:** any tool call that succeeds (shape shown with Bash `echo "hello from bash"; pwd`)

--- RAW OUTPUT START ---
[result]
[name]Bash[/name]
[output]hello from bash
/c/AI/MOSAIC/MOSAIC[/output]
[/result]
--- RAW OUTPUT END ---

## Wrapper — Failed tool result

**Call:** any tool call that fails (shape shown with Read on a missing file)

--- RAW OUTPUT START ---
[result]
[name]Read[/name]
[error]File does not exist. Note: your current working directory is C:\AI\MOSAIC\MOSAIC.[/error]
[/result]
--- RAW OUTPUT END ---

## Read — Full file (varied-content-test.txt)

**Call:** `Read({file_path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt"})`

--- RAW OUTPUT START ---
1	Line 1: Greetings, Universe! 🚀
2	Line 2: This file has "double quotes" and 'single quotes' for validation.
3	Line 3:     Four-space indentation here.
4	Line 4:	Tab-indented content here.
5	Line 5: Special characters: <chevrons> & ampersand | pipe \ backslash
6	Line 6: Unicode: resume, naive, 中文测试, Кириллица, עברית
7	Line 7: Blank line follows:
8	
9	Line 9: Resumed after blank line.
10	Line 10: Path-like content: /opt/tools/bin/run --mode=capture
11	Line 11: Code-like content: const calc = (x) => { return x > 0 && x < 100; }
12	Line 12: Regex-like content: ^begin.*center[a-z]+finish$
13	Line 13: XML-like content: <node role="test">nested value</node>
14	Line 14: JSON-like content: {"name": "probe", "count": 77, "items": [4, 5, 6]}
15	--- EDIT TARGET START ---
16	Line 16: Swap this line with different content.
17	--- EDIT TARGET END ---
18	Line 18: This line must stay intact after edit tests.
19	Line 19: Grep target: DISTINCT_MATCH_TOKEN_ABC
20	Line 20: Final line with trailing newline.
21	
--- RAW OUTPUT END ---

## Read — Partial file (offset 5, limit 3)

**Call:** `Read({file_path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt", offset: 5, limit: 3})`

--- RAW OUTPUT START ---
5	Line 5: Special characters: <chevrons> & ampersand | pipe \ backslash
6	Line 6: Unicode: resume, naive, 中文测试, Кириллица, עברית
7	Line 7: Blank line follows:
--- RAW OUTPUT END ---

## Read — Directory (error)

**Call:** `Read({file_path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts"})`

--- RAW OUTPUT START ---
[error]EISDIR: illegal operation on a directory, read 'C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts'[/error]
--- RAW OUTPUT END ---

## Read — File not found (error)

**Call:** `Read({file_path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\does-not-exist.txt"})`

--- RAW OUTPUT START ---
[error]File does not exist. Note: your current working directory is C:\AI\MOSAIC\MOSAIC.[/error]
--- RAW OUTPUT END ---

## Read — Large file, default parameters (large-file-3000-lines.txt)

**Call:** `Read({file_path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\large-file-3000-lines.txt"})`

--- RAW OUTPUT START ---
1	foxtrot golf [ROW:0001] hotel india
2	foxtrot golf [ROW:0002] hotel india
3	foxtrot golf [ROW:0003] hotel india
4	foxtrot golf [ROW:0004] hotel india
5	foxtrot golf [ROW:0005] hotel india
6	foxtrot golf [ROW:0006] hotel india
7	foxtrot golf [ROW:0007] hotel india
8	foxtrot golf [ROW:0008] hotel india
9	foxtrot golf [ROW:0009] hotel india
10	foxtrot golf [ROW:0010] hotel india
11	foxtrot golf [ROW:0011] hotel india
12	foxtrot golf [ROW:0012] hotel india
13	foxtrot golf [ROW:0013] hotel india
14	foxtrot golf [ROW:0014] hotel india
15	foxtrot golf [ROW:0015] hotel india
16	foxtrot golf [ROW:0016] hotel india
17	foxtrot golf [ROW:0017] hotel india
18	foxtrot golf [ROW:0018] hotel india
19	foxtrot golf [ROW:0019] hotel india
20	foxtrot golf [ROW:0020] hotel india
21	foxtrot golf [ROW:0021] hotel india
22	foxtrot golf [ROW:0022] hotel india
23	foxtrot golf [ROW:0023] hotel india
24	foxtrot golf [ROW:0024] hotel india
25	foxtrot golf [ROW:0025] hotel india
26	foxtrot golf [ROW:0026] hotel india
27	foxtrot golf [ROW:0027] hotel india
28	foxtrot golf [ROW:0028] hotel india
29	foxtrot golf [ROW:0029] hotel india
30	foxtrot golf [ROW:0030] hotel india
31	foxtrot golf [ROW:0031] hotel india
32	foxtrot golf [ROW:0032] hotel india
33	foxtrot golf [ROW:0033] hotel india
34	foxtrot golf [ROW:0034] hotel india
35	foxtrot golf [ROW:0035] hotel india
36	foxtrot golf [ROW:0036] hotel india
37	foxtrot golf [ROW:0037] hotel india
38	foxtrot golf [ROW:0038] hotel india
39	foxtrot golf [ROW:0039] hotel india
40	foxtrot golf [ROW:0040] hotel india
41	foxtrot golf [ROW:0041] hotel india
42	foxtrot golf [ROW:0042] hotel india
43	foxtrot golf [ROW:0043] hotel india
44	foxtrot golf [ROW:0044] hotel india
45	foxtrot golf [ROW:0045] hotel india
46	foxtrot golf [ROW:0046] hotel india
47	foxtrot golf [ROW:0047] hotel india
48	foxtrot golf [ROW:0048] hotel india
49	foxtrot golf [ROW:0049] hotel india
50	foxtrot golf [ROW:0050] hotel india
51	foxtrot golf [ROW:0051] hotel india
52	foxtrot golf [ROW:0052] hotel india
53	foxtrot golf [ROW:0053] hotel india
54	foxtrot golf [ROW:0054] hotel india
55	foxtrot golf [ROW:0055] hotel india
56	foxtrot golf [ROW:0056] hotel india
57	foxtrot golf [ROW:0057] hotel india
58	foxtrot golf [ROW:0058] hotel india
59	foxtrot golf [ROW:0059] hotel india
60	foxtrot golf [ROW:0060] hotel india
61	foxtrot golf [ROW:0061] hotel india
62	foxtrot golf [ROW:0062] hotel india
63	foxtrot golf [ROW:0063] hotel india
64	foxtrot golf [ROW:0064] hotel india
65	foxtrot golf [ROW:0065] hotel india
66	foxtrot golf [ROW:0066] hotel india
67	foxtrot golf [ROW:0067] hotel india
68	foxtrot golf [ROW:0068] hotel india
69	foxtrot golf [ROW:0069] hotel india
70	foxtrot golf [ROW:0070] hotel india
71	foxtrot golf [ROW:0071] hotel india
72	foxtrot golf [ROW:0072] hotel india
73	foxtrot golf [ROW:0073] hotel india
74	foxtrot golf [ROW:0074] hotel india
75	foxtrot golf [ROW:0075] hotel india
76	foxtrot golf [ROW:0076] hotel india
77	foxtrot golf [ROW:0077] hotel india
78	foxtrot golf [ROW:0078] hotel india
79	foxtrot golf [ROW:0079] hotel india
80	foxtrot golf [ROW:0080] hotel india
81	foxtrot golf [ROW:0081] hotel india
82	foxtrot golf [ROW:0082] hotel india
83	foxtrot golf [ROW:0083] hotel india
84	foxtrot golf [ROW:0084] hotel india
85	foxtrot golf [ROW:0085] hotel india
86	foxtrot golf [ROW:0086] hotel india
87	foxtrot golf [ROW:0087] hotel india
88	foxtrot golf [ROW:0088] hotel india
89	foxtrot golf [ROW:0089] hotel india
90	foxtrot golf [ROW:0090] hotel india
91	foxtrot golf [ROW:0091] hotel india
92	foxtrot golf [ROW:0092] hotel india
93	foxtrot golf [ROW:0093] hotel india
94	foxtrot golf [ROW:0094] hotel india
95	foxtrot golf [ROW:0095] hotel india
96	foxtrot golf [ROW:0096] hotel india
97	foxtrot golf [ROW:0097] hotel india
98	foxtrot golf [ROW:0098] hotel india
99	foxtrot golf [ROW:0099] hotel india
100	foxtrot golf [ROW:0100] hotel india
101	foxtrot golf [ROW:0101] hotel india
102	foxtrot golf [ROW:0102] hotel india
103	foxtrot golf [ROW:0103] hotel india
104	foxtrot golf [ROW:0104] hotel india
105	foxtrot golf [ROW:0105] hotel india
106	foxtrot golf [ROW:0106] hotel india
107	foxtrot golf [ROW:0107] hotel india
108	foxtrot golf [ROW:0108] hotel india
109	foxtrot golf [ROW:0109] hotel india
110	foxtrot golf [ROW:0110] hotel india
111	foxtrot golf [ROW:0111] hotel india
112	foxtrot golf [ROW:0112] hotel india
113	foxtrot golf [ROW:0113] hotel india
114	foxtrot golf [ROW:0114] hotel india
115	foxtrot golf [ROW:0115] hotel india
116	foxtrot golf [ROW:0116] hotel india
117	foxtrot golf [ROW:0117] hotel india
118	foxtrot golf [ROW:0118] hotel india
119	foxtrot golf [ROW:0119] hotel india
120	foxtrot golf [ROW:0120] hotel india
121	foxtrot golf [ROW:0121] hotel india
122	foxtrot golf [ROW:0122] hotel india
123	foxtrot golf [ROW:0123] hotel india
124	foxtrot golf [ROW:0124] hotel india
125	foxtrot golf [ROW:0125] hotel india
126	foxtrot golf [ROW:0126] hotel india
127	foxtrot golf [ROW:0127] hotel india
128	foxtrot golf [ROW:0128] hotel india
129	foxtrot golf [ROW:0129] hotel india
130	foxtrot golf [ROW:0130] hotel india
131	foxtrot golf [ROW:0131] hotel india
132	foxtrot golf [ROW:0132] hotel india
133	foxtrot golf [ROW:0133] hotel india
134	foxtrot golf [ROW:0134] hotel india
135	foxtrot golf [ROW:0135] hotel india
136	foxtrot golf [ROW:0136] hotel india
137	foxtrot golf [ROW:0137] hotel india
138	foxtrot golf [ROW:0138] hotel india
139	foxtrot golf [ROW:0139] hotel india
140	foxtrot golf [ROW:0140] hotel india
141	foxtrot golf [ROW:0141] hotel india
142	foxtrot golf [ROW:0142] hotel india
143	foxtrot golf [ROW:0143] hotel india
144	foxtrot golf [ROW:0144] hotel india
145	foxtrot golf [ROW:0145] hotel india
146	foxtrot golf [ROW:0146] hotel india
147	foxtrot golf [ROW:0147] hotel india
148	foxtrot golf [ROW:0148] hotel india
149	foxtrot golf [ROW:0149] hotel india
150	foxtrot golf [ROW:0150] hotel india
151	foxtrot golf [ROW:0151] hotel india
152	foxtrot golf [ROW:0152] hotel india
153	foxtrot golf [ROW:0153] hotel india
154	foxtrot golf [ROW:0154] hotel india
155	foxtrot golf [ROW:0155] hotel india
156	foxtrot golf [ROW:0156] hotel india
157	foxtrot golf [ROW:0157] hotel india
158	foxtrot golf [ROW:0158] hotel india
159	foxtrot golf [ROW:0159] hotel india
160	foxtrot golf [ROW:0160] hotel india
161	foxtrot golf [ROW:0161] hotel india
162	foxtrot golf [ROW:0162] hotel india
163	foxtrot golf [ROW:0163] hotel india
164	foxtrot golf [ROW:0164] hotel india
165	foxtrot golf [ROW:0165] hotel india
166	foxtrot golf [ROW:0166] hotel india
167	foxtrot golf [ROW:0167] hotel india
168	foxtrot golf [ROW:0168] hotel india
169	foxtrot golf [ROW:0169] hotel india
170	foxtrot golf [ROW:0170] hotel india
171	foxtrot golf [ROW:0171] hotel india
172	foxtrot golf [ROW:0172] hotel india
173	foxtrot golf [ROW:0173] hotel india
174	foxtrot golf [ROW:0174] hotel india
175	foxtrot golf [ROW:0175] hotel india
176	foxtrot golf [ROW:0176] hotel india
177	foxtrot golf [ROW:0177] hotel india
178	foxtrot golf [ROW:0178] hotel india
179	foxtrot golf [ROW:0179] hotel india
180	foxtrot golf [ROW:0180] hotel india
181	foxtrot golf [ROW:0181] hotel india
182	foxtrot golf [ROW:0182] hotel india
183	foxtrot golf [ROW:0183] hotel india
184	foxtrot golf [ROW:0184] hotel india
185	foxtrot golf [ROW:0185] hotel india
186	foxtrot golf [ROW:0186] hotel india
187	foxtrot golf [ROW:0187] hotel india
188	foxtrot golf [ROW:0188] hotel india
189	foxtrot golf [ROW:0189] hotel india
190	foxtrot golf [ROW:0190] hotel india
191	foxtrot golf [ROW:0191] hotel india
192	foxtrot golf [ROW:0192] hotel india
193	foxtrot golf [ROW:0193] hotel india
194	foxtrot golf [ROW:0194] hotel india
195	foxtrot golf [ROW:0195] hotel india
196	foxtrot golf [ROW:0196] hotel india
197	foxtrot golf [ROW:0197] hotel india
198	foxtrot golf [ROW:0198] hotel india
199	foxtrot golf [ROW:0199] hotel india
200	foxtrot golf [ROW:0200] hotel india
201	foxtrot golf [ROW:0201] hotel india
202	foxtrot golf [ROW:0202] hotel india
203	foxtrot golf [ROW:0203] hotel india
204	foxtrot golf [ROW:0204] hotel india
205	foxtrot golf [ROW:0205] hotel india
206	foxtrot golf [ROW:0206] hotel india
207	foxtrot golf [ROW:0207] hotel india
208	foxtrot golf [ROW:0208] hotel india
209	foxtrot golf [ROW:0209] hotel india
210	foxtrot golf [ROW:0210] hotel india
211	foxtrot golf [ROW:0211] hotel india
212	foxtrot golf [ROW:0212] hotel india
213	foxtrot golf [ROW:0213] hotel india
214	foxtrot golf [ROW:0214] hotel india
215	foxtrot golf [ROW:0215] hotel india
216	foxtrot golf [ROW:0216] hotel india
217	foxtrot golf [ROW:0217] hotel india
218	foxtrot golf [ROW:0218] hotel india
219	foxtrot golf [ROW:0219] hotel india
220	foxtrot golf [ROW:0220] hotel india
221	foxtrot golf [ROW:0221] hotel india
222	foxtrot golf [ROW:0222] hotel india
223	foxtrot golf [ROW:0223] hotel india
224	foxtrot golf [ROW:0224] hotel india
225	foxtrot golf [ROW:0225] hotel india
226	foxtrot golf [ROW:0226] hotel india
227	foxtrot golf [ROW:0227] hotel india
228	foxtrot golf [ROW:0228] hotel india
229	foxtrot golf [ROW:0229] hotel india
230	foxtrot golf [ROW:0230] hotel india
231	foxtrot golf [ROW:0231] hotel india
232	foxtrot golf [ROW:0232] hotel india
233	foxtrot golf [ROW:0233] hotel india
234	foxtrot golf [ROW:0234] hotel india
235	foxtrot golf [ROW:0235] hotel india
236	foxtrot golf [ROW:0236] hotel india
237	foxtrot golf [ROW:0237] hotel india
238	foxtrot golf [ROW:0238] hotel india
239	foxtrot golf [ROW:0239] hotel india
240	foxtrot golf [ROW:0240] hotel india
241	foxtrot golf [ROW:0241] hotel india
242	foxtrot golf [ROW:0242] hotel india
243	foxtrot golf [ROW:0243] hotel india
244	foxtrot golf [ROW:0244] hotel india
245	foxtrot golf [ROW:0245] hotel india
246	foxtrot golf [ROW:0246] hotel india
247	foxtrot golf [ROW:0247] hotel india
248	foxtrot golf [ROW:0248] hotel india
249	foxtrot golf [ROW:0249] hotel india
250	foxtrot golf [ROW:0250] hotel india
251	foxtrot golf [ROW:0251] hotel india
252	foxtrot golf [ROW:0252] hotel india
253	foxtrot golf [ROW:0253] hotel india
254	foxtrot golf [ROW:0254] hotel india
255	foxtrot golf [ROW:0255] hotel india
256	foxtrot golf [ROW:0256] hotel india
257	foxtrot golf [ROW:0257] hotel india
258	foxtrot golf [ROW:0258] hotel india
259	foxtrot golf [ROW:0259] hotel india
260	foxtrot golf [ROW:0260] hotel india
261	foxtrot golf [ROW:0261] hotel india
262	foxtrot golf [ROW:0262] hotel india
263	foxtrot golf [ROW:0263] hotel india
264	foxtrot golf [ROW:0264] hotel india
265	foxtrot golf [ROW:0265] hotel india
266	foxtrot golf [ROW:0266] hotel india
267	foxtrot golf [ROW:0267] hotel india
268	foxtrot golf [ROW:0268] hotel india
269	foxtrot golf [ROW:0269] hotel india
270	foxtrot golf [ROW:0270] hotel india
271	foxtrot golf [ROW:0271] hotel india
272	foxtrot golf [ROW:0272] hotel india
273	foxtrot golf [ROW:0273] hotel india
274	foxtrot golf [ROW:0274] hotel india
275	foxtrot golf [ROW:0275] hotel india
276	foxtrot golf [ROW:0276] hotel india
277	foxtrot golf [ROW:0277] hotel india
278	foxtrot golf [ROW:0278] hotel india
279	foxtrot golf [ROW:0279] hotel india
280	foxtrot golf [ROW:0280] hotel india
281	foxtrot golf [ROW:0281] hotel india
282	foxtrot golf [ROW:0282] hotel india
283	foxtrot golf [ROW:0283] hotel india
284	foxtrot golf [ROW:0284] hotel india
285	foxtrot golf [ROW:0285] hotel india
286	foxtrot golf [ROW:0286] hotel india
287	foxtrot golf [ROW:0287] hotel india
288	foxtrot golf [ROW:0288] hotel india
289	foxtrot golf [ROW:0289] hotel india
290	foxtrot golf [ROW:0290] hotel india
291	foxtrot golf [ROW:0291] hotel india
292	foxtrot golf [ROW:0292] hotel india
293	foxtrot golf [ROW:0293] hotel india
294	foxtrot golf [ROW:0294] hotel india
295	foxtrot golf [ROW:0295] hotel india
296	foxtrot golf [ROW:0296] hotel india
297	foxtrot golf [ROW:0297] hotel india
298	foxtrot golf [ROW:0298] hotel india
299	foxtrot golf [ROW:0299] hotel india
300	foxtrot golf [ROW:0300] hotel india
301	foxtrot golf [ROW:0301] hotel india
302	foxtrot golf [ROW:0302] hotel india
303	foxtrot golf [ROW:0303] hotel india
304	foxtrot golf [ROW:0304] hotel india
305	foxtrot golf [ROW:0305] hotel india
306	foxtrot golf [ROW:0306] hotel india
307	foxtrot golf [ROW:0307] hotel india
308	foxtrot golf [ROW:0308] hotel india
309	foxtrot golf [ROW:0309] hotel india
310	foxtrot golf [ROW:0310] hotel india
311	foxtrot golf [ROW:0311] hotel india
312	foxtrot golf [ROW:0312] hotel india
313	foxtrot golf [ROW:0313] hotel india
314	foxtrot golf [ROW:0314] hotel india
315	foxtrot golf [ROW:0315] hotel india
316	foxtrot golf [ROW:0316] hotel india
317	foxtrot golf [ROW:0317] hotel india
318	foxtrot golf [ROW:0318] hotel india
319	foxtrot golf [ROW:0319] hotel india
320	foxtrot golf [ROW:0320] hotel india
321	foxtrot golf [ROW:0321] hotel india
322	foxtrot golf [ROW:0322] hotel india
323	foxtrot golf [ROW:0323] hotel india
324	foxtrot golf [ROW:0324] hotel india
325	foxtrot golf [ROW:0325] hotel india
326	foxtrot golf [ROW:0326] hotel india
327	foxtrot golf [ROW:0327] hotel india
328	foxtrot golf [ROW:0328] hotel india
329	foxtrot golf [ROW:0329] hotel india
330	foxtrot golf [ROW:0330] hotel india
331	foxtrot golf [ROW:0331] hotel india
332	foxtrot golf [ROW:0332] hotel india
333	foxtrot golf [ROW:0333] hotel india
334	foxtrot golf [ROW:0334] hotel india
335	foxtrot golf [ROW:0335] hotel india
336	foxtrot golf [ROW:0336] hotel india
337	foxtrot golf [ROW:0337] hotel india
338	foxtrot golf [ROW:0338] hotel india
339	foxtrot golf [ROW:0339] hotel india
340	foxtrot golf [ROW:0340] hotel india
341	foxtrot golf [ROW:0341] hotel india
342	foxtrot golf [ROW:0342] hotel india
343	foxtrot golf [ROW:0343] hotel india
344	foxtrot golf [ROW:0344] hotel india
345	foxtrot golf [ROW:0345] hotel india
346	foxtrot golf [ROW:0346] hotel india
347	foxtrot golf [ROW:0347] hotel india
348	foxtrot golf [ROW:0348] hotel india
349	foxtrot golf [ROW:0349] hotel india
350	foxtrot golf [ROW:0350] hotel india
351	foxtrot golf [ROW:0351] hotel india
352	foxtrot golf [ROW:0352] hotel india
353	foxtrot golf [ROW:0353] hotel india
354	foxtrot golf [ROW:0354] hotel india
355	foxtrot golf [ROW:0355] hotel india
356	foxtrot golf [ROW:0356] hotel india
357	foxtrot golf [ROW:0357] hotel india
358	foxtrot golf [ROW:0358] hotel india
359	foxtrot golf [ROW:0359] hotel india
360	foxtrot golf [ROW:0360] hotel india
361	foxtrot golf [ROW:0361] hotel india
362	foxtrot golf [ROW:0362] hotel india
363	foxtrot golf [ROW:0363] hotel india
364	foxtrot golf [ROW:0364] hotel india
365	foxtrot golf [ROW:0365] hotel india
366	foxtrot golf [ROW:0366] hotel india
367	foxtrot golf [ROW:0367] hotel india
368	foxtrot golf [ROW:0368] hotel india
369	foxtrot golf [ROW:0369] hotel india
370	foxtrot golf [ROW:0370] hotel india
371	foxtrot golf [ROW:0371] hotel india
372	foxtrot golf [ROW:0372] hotel india
373	foxtrot golf [ROW:0373] hotel india
374	foxtrot golf [ROW:0374] hotel india
375	foxtrot golf [ROW:0375] hotel india
376	foxtrot golf [ROW:0376] hotel india
377	foxtrot golf [ROW:0377] hotel india
378	foxtrot golf [ROW:0378] hotel india
379	foxtrot golf [ROW:0379] hotel india
380	foxtrot golf [ROW:0380] hotel india
381	foxtrot golf [ROW:0381] hotel india
382	foxtrot golf [ROW:0382] hotel india
383	foxtrot golf [ROW:0383] hotel india
384	foxtrot golf [ROW:0384] hotel india
385	foxtrot golf [ROW:0385] hotel india
386	foxtrot golf [ROW:0386] hotel india
387	foxtrot golf [ROW:0387] hotel india
388	foxtrot golf [ROW:0388] hotel india
389	foxtrot golf [ROW:0389] hotel india
390	foxtrot golf [ROW:0390] hotel india
391	foxtrot golf [ROW:0391] hotel india
392	foxtrot golf [ROW:0392] hotel india
393	foxtrot golf [ROW:0393] hotel india
394	foxtrot golf [ROW:0394] hotel india
395	foxtrot golf [ROW:0395] hotel india
396	foxtrot golf [ROW:0396] hotel india
397	foxtrot golf [ROW:0397] hotel india
398	foxtrot golf [ROW:0398] hotel india
399	foxtrot golf [ROW:0399] hotel india
400	foxtrot golf [ROW:0400] hotel india
401	foxtrot golf [ROW:0401] hotel india
402	foxtrot golf [ROW:0402] hotel india
403	foxtrot golf [ROW:0403] hotel india
404	foxtrot golf [ROW:0404] hotel india
405	foxtrot golf [ROW:0405] hotel india
406	foxtrot golf [ROW:0406] hotel india
407	foxtrot golf [ROW:0407] hotel india
408	foxtrot golf [ROW:0408] hotel india
409	foxtrot golf [ROW:0409] hotel india
410	foxtrot golf [ROW:0410] hotel india
411	foxtrot golf [ROW:0411] hotel india
412	foxtrot golf [ROW:0412] hotel india
413	foxtrot golf [ROW:0413] hotel india
414	foxtrot golf [ROW:0414] hotel india
415	foxtrot golf [ROW:0415] hotel india
416	foxtrot golf [ROW:0416] hotel india
417	foxtrot golf [ROW:0417] hotel india
418	foxtrot golf [ROW:0418] hotel india
419	foxtrot golf [ROW:0419] hotel india
420	foxtrot golf [ROW:0420] hotel india
421	foxtrot golf [ROW:0421] hotel india
422	foxtrot golf [ROW:0422] hotel india
423	foxtrot golf [ROW:0423] hotel india
424	foxtrot golf [ROW:0424] hotel india
425	foxtrot golf [ROW:0425] hotel india
426	foxtrot golf [ROW:0426] hotel india
427	foxtrot golf [ROW:0427] hotel india
428	foxtrot golf [ROW:0428] hotel india
429	foxtrot golf [ROW:0429] hotel india
430	foxtrot golf [ROW:0430] hotel india
431	foxtrot golf [ROW:0431] hotel india
432	foxtrot golf [ROW:0432] hotel india
433	foxtrot golf [ROW:0433] hotel india
434	foxtrot golf [ROW:0434] hotel india
435	foxtrot golf [ROW:0435] hotel india
436	foxtrot golf [ROW:0436] hotel india
437	foxtrot golf [ROW:0437] hotel india
438	foxtrot golf [ROW:0438] hotel india
439	foxtrot golf [ROW:0439] hotel india
440	foxtrot golf [ROW:0440] hotel india
441	foxtrot golf [ROW:0441] hotel india
442	foxtrot golf [ROW:0442] hotel india
443	foxtrot golf [ROW:0443] hotel india
444	foxtrot golf [ROW:0444] hotel india
445	foxtrot golf [ROW:0445] hotel india
446	foxtrot golf [ROW:0446] hotel india
447	foxtrot golf [ROW:0447] hotel india
448	foxtrot golf [ROW:0448] hotel india
449	foxtrot golf [ROW:0449] hotel india
450	foxtrot golf [ROW:0450] hotel india
451	foxtrot golf [ROW:0451] hotel india
452	foxtrot golf [ROW:0452] hotel india
453	foxtrot golf [ROW:0453] hotel india
454	foxtrot golf [ROW:0454] hotel india
455	foxtrot golf [ROW:0455] hotel india
456	foxtrot golf [ROW:0456] hotel india
457	foxtrot golf [ROW:0457] hotel india
458	foxtrot golf [ROW:0458] hotel india
459	foxtrot golf [ROW:0459] hotel india
460	foxtrot golf [ROW:0460] hotel india
461	foxtrot golf [ROW:0461] hotel india
462	foxtrot golf [ROW:0462] hotel india
463	foxtrot golf [ROW:0463] hotel india
464	foxtrot golf [ROW:0464] hotel india
465	foxtrot golf [ROW:0465] hotel india
466	foxtrot golf [ROW:0466] hotel india
467	foxtrot golf [ROW:0467] hotel india
468	foxtrot golf [ROW:0468] hotel india
469	foxtrot golf [ROW:0469] hotel india
470	foxtrot golf [ROW:0470] hotel india
471	foxtrot golf [ROW:0471] hotel india
472	foxtrot golf [ROW:0472] hotel india
473	foxtrot golf [ROW:0473] hotel india
474	foxtrot golf [ROW:0474] hotel india
475	foxtrot golf [ROW:0475] hotel india
476	foxtrot golf [ROW:0476] hotel india
477	foxtrot golf [ROW:0477] hotel india
478	foxtrot golf [ROW:0478] hotel india
479	foxtrot golf [ROW:0479] hotel india
480	foxtrot golf [ROW:0480] hotel india
481	foxtrot golf [ROW:0481] hotel india
482	foxtrot golf [ROW:0482] hotel india
483	foxtrot golf [ROW:0483] hotel india
484	foxtrot golf [ROW:0484] hotel india
485	foxtrot golf [ROW:0485] hotel india
486	foxtrot golf [ROW:0486] hotel india
487	foxtrot golf [ROW:0487] hotel india
488	foxtrot golf [ROW:0488] hotel india
489	foxtrot golf [ROW:0489] hotel india
490	foxtrot golf [ROW:0490] hotel india
491	foxtrot golf [ROW:0491] hotel india
492	foxtrot golf [ROW:0492] hotel india
493	foxtrot golf [ROW:0493] hotel india
494	foxtrot golf [ROW:0494] hotel india
495	foxtrot golf [ROW:0495] hotel india
496	foxtrot golf [ROW:0496] hotel india
497	foxtrot golf [ROW:0497] hotel india
498	foxtrot golf [ROW:0498] hotel india
499	foxtrot golf [ROW:0499] hotel india
500	foxtrot golf [ROW:0500] hotel india
501	foxtrot golf [ROW:0501] hotel india
502	foxtrot golf [ROW:0502] hotel india
503	foxtrot golf [ROW:0503] hotel india
504	foxtrot golf [ROW:0504] hotel india
505	foxtrot golf [ROW:0505] hotel india
506	foxtrot golf [ROW:0506] hotel india
507	foxtrot golf [ROW:0507] hotel india
508	foxtrot golf [ROW:0508] hotel india
509	foxtrot golf [ROW:0509] hotel india
510	foxtrot golf [ROW:0510] hotel india
511	foxtrot golf [ROW:0511] hotel india
512	foxtrot golf [ROW:0512] hotel india
513	foxtrot golf [ROW:0513] hotel india
514	foxtrot golf [ROW:0514] hotel india
515	foxtrot golf [ROW:0515] hotel india
516	foxtrot golf [ROW:0516] hotel india
517	foxtrot golf [ROW:0517] hotel india
518	foxtrot golf [ROW:0518] hotel india
519	foxtrot golf [ROW:0519] hotel india
520	foxtrot golf [ROW:0520] hotel india
521	foxtrot golf [ROW:0521] hotel india
522	foxtrot golf [ROW:0522] hotel india
523	foxtrot golf [ROW:0523] hotel india
524	foxtrot golf [ROW:0524] hotel india
525	foxtrot golf [ROW:0525] hotel india
526	foxtrot golf [ROW:0526] hotel india
527	foxtrot golf [ROW:0527] hotel india
528	foxtrot golf [ROW:0528] hotel india
529	foxtrot golf [ROW:0529] hotel india
530	foxtrot golf [ROW:0530] hotel india
531	foxtrot golf [ROW:0531] hotel india
532	foxtrot golf [ROW:0532] hotel india
533	foxtrot golf [ROW:0533] hotel india
534	foxtrot golf [ROW:0534] hotel india
535	foxtrot golf [ROW:0535] hotel india
536	foxtrot golf [ROW:0536] hotel india
537	foxtrot golf [ROW:0537] hotel india
538	foxtrot golf [ROW:0538] hotel india
539	foxtrot golf [ROW:0539] hotel india
540	foxtrot golf [ROW:0540] hotel india
541	foxtrot golf [ROW:0541] hotel india
542	foxtrot golf [ROW:0542] hotel india
543	foxtrot golf [ROW:0543] hotel india
544	foxtrot golf [ROW:0544] hotel india
545	foxtrot golf [ROW:0545] hotel india
546	foxtrot golf [ROW:0546] hotel india
547	foxtrot golf [ROW:0547] hotel india
548	foxtrot golf [ROW:0548] hotel india
549	foxtrot golf [ROW:0549] hotel india
550	foxtrot golf [ROW:0550] hotel india
551	foxtrot golf [ROW:0551] hotel india
552	foxtrot golf [ROW:0552] hotel india
553	foxtrot golf [ROW:0553] hotel india
554	foxtrot golf [ROW:0554] hotel india
555	foxtrot golf [ROW:0555] hotel india
556	foxtrot golf [ROW:0556] hotel india
557	foxtrot golf [ROW:0557] hotel india
558	foxtrot golf [ROW:0558] hotel india
559	foxtrot golf [ROW:0559] hotel india
560	foxtrot golf [ROW:0560] hotel india
561	foxtrot golf [ROW:0561] hotel india
562	foxtrot golf [ROW:0562] hotel india
563	foxtrot golf [ROW:0563] hotel india
564	foxtrot golf [ROW:0564] hotel india
565	foxtrot golf [ROW:0565] hotel india
566	foxtrot golf [ROW:0566] hotel india
567	foxtrot golf [ROW:0567] hotel india
568	foxtrot golf [ROW:0568] hotel india
569	foxtrot golf [ROW:0569] hotel india
570	foxtrot golf [ROW:0570] hotel india
571	foxtrot golf [ROW:0571] hotel india
572	foxtrot golf [ROW:0572] hotel india
573	foxtrot golf [ROW:0573] hotel india
574	foxtrot golf [ROW:0574] hotel india
575	foxtrot golf [ROW:0575] hotel india
576	foxtrot golf [ROW:0576] hotel india
577	foxtrot golf [ROW:0577] hotel india
578	foxtrot golf [ROW:0578] hotel india
579	foxtrot golf [ROW:0579] hotel india
580	foxtrot golf [ROW:0580] hotel india
581	foxtrot golf [ROW:0581] hotel india
582	foxtrot golf [ROW:0582] hotel india
583	foxtrot golf [ROW:0583] hotel india
584	foxtrot golf [ROW:0584] hotel india
585	foxtrot golf [ROW:0585] hotel india
586	foxtrot golf [ROW:0586] hotel india
587	foxtrot golf [ROW:0587] hotel india
588	foxtrot golf [ROW:0588] hotel india
589	foxtrot golf [ROW:0589] hotel india
590	foxtrot golf [ROW:0590] hotel india
591	foxtrot golf [ROW:0591] hotel india
592	foxtrot golf [ROW:0592] hotel india
593	foxtrot golf [ROW:0593] hotel india
594	foxtrot golf [ROW:0594] hotel india
595	foxtrot golf [ROW:0595] hotel india
596	foxtrot golf [ROW:0596] hotel india
597	foxtrot golf [ROW:0597] hotel india
598	foxtrot golf [ROW:0598] hotel india
599	foxtrot golf [ROW:0599] hotel india
600	foxtrot golf [ROW:0600] hotel india
601	foxtrot golf [ROW:0601] hotel india
602	foxtrot golf [ROW:0602] hotel india
603	foxtrot golf [ROW:0603] hotel india
604	foxtrot golf [ROW:0604] hotel india
605	foxtrot golf [ROW:0605] hotel india
606	foxtrot golf [ROW:0606] hotel india
607	foxtrot golf [ROW:0607] hotel india
608	foxtrot golf [ROW:0608] hotel india
609	foxtrot golf [ROW:0609] hotel india
610	foxtrot golf [ROW:0610] hotel india
611	foxtrot golf [ROW:0611] hotel india
612	foxtrot golf [ROW:0612] hotel india
613	foxtrot golf [ROW:0613] hotel india
614	foxtrot golf [ROW:0614] hotel india
615	foxtrot golf [ROW:0615] hotel india
616	foxtrot golf [ROW:0616] hotel india
617	foxtrot golf [ROW:0617] hotel india
618	foxtrot golf [ROW:0618] hotel india
619	foxtrot golf [ROW:0619] hotel india
620	foxtrot golf [ROW:0620] hotel india
621	foxtrot golf [ROW:0621] hotel india
622	foxtrot golf [ROW:0622] hotel india
623	foxtrot golf [ROW:0623] hotel india
624	foxtrot golf [ROW:0624] hotel india
625	foxtrot golf [ROW:0625] hotel india
626	foxtrot golf [ROW:0626] hotel india
627	foxtrot golf [ROW:0627] hotel india
628	foxtrot golf [ROW:0628] hotel india
629	foxtrot golf [ROW:0629] hotel india
630	foxtrot golf [ROW:0630] hotel india
631	foxtrot golf [ROW:0631] hotel india
632	foxtrot golf [ROW:0632] hotel india
633	foxtrot golf [ROW:0633] hotel india
634	foxtrot golf [ROW:0634] hotel india
635	foxtrot golf [ROW:0635] hotel india
636	foxtrot golf [ROW:0636] hotel india
637	foxtrot golf [ROW:0637] hotel india
638	foxtrot golf [ROW:0638] hotel india
639	foxtrot golf [ROW:0639] hotel india
640	foxtrot golf [ROW:0640] hotel india
641	foxtrot golf [ROW:0641] hotel india
642	foxtrot golf [ROW:0642] hotel india
643	foxtrot golf [ROW:0643] hotel india
644	foxtrot golf [ROW:0644] hotel india
645	foxtrot golf [ROW:0645] hotel india
646	foxtrot golf [ROW:0646] hotel india
647	foxtrot golf [ROW:0647] hotel india
648	foxtrot golf [ROW:0648] hotel india
649	foxtrot golf [ROW:0649] hotel india
650	foxtrot golf [ROW:0650] hotel india
651	foxtrot golf [ROW:0651] hotel india
652	foxtrot golf [ROW:0652] hotel india
653	foxtrot golf [ROW:0653] hotel india
654	foxtrot golf [ROW:0654] hotel india
655	foxtrot golf [ROW:0655] hotel india
656	foxtrot golf [ROW:0656] hotel india
657	foxtrot golf [ROW:0657] hotel india
658	foxtrot golf [ROW:0658] hotel india
659	foxtrot golf [ROW:0659] hotel india
660	foxtrot golf [ROW:0660] hotel india
661	foxtrot golf [ROW:0661] hotel india
662	foxtrot golf [ROW:0662] hotel india
663	foxtrot golf [ROW:0663] hotel india
664	foxtrot golf [ROW:0664] hotel india
665	foxtrot golf [ROW:0665] hotel india
666	foxtrot golf [ROW:0666] hotel india
667	foxtrot golf [ROW:0667] hotel india
668	foxtrot golf [ROW:0668] hotel india
669	foxtrot golf [ROW:0669] hotel india
670	foxtrot golf [ROW:0670] hotel india
671	foxtrot golf [ROW:0671] hotel india
672	foxtrot golf [ROW:0672] hotel india
673	foxtrot golf [ROW:0673] hotel india
674	foxtrot golf [ROW:0674] hotel india
675	foxtrot golf [ROW:0675] hotel india
676	foxtrot golf [ROW:0676] hotel india
677	foxtrot golf [ROW:0677] hotel india
678	foxtrot golf [ROW:0678] hotel india
679	foxtrot golf [ROW:0679] hotel india
680	foxtrot golf [ROW:0680] hotel india
681	foxtrot golf [ROW:0681] hotel india
682	foxtrot golf [ROW:0682] hotel india
683	foxtrot golf [ROW:0683] hotel india
684	foxtrot golf [ROW:0684] hotel india
685	foxtrot golf [ROW:0685] hotel india
686	foxtrot golf [ROW:0686] hotel india
687	foxtrot golf [ROW:0687] hotel india
688	foxtrot golf [ROW:0688] hotel india
689	foxtrot golf [ROW:0689] hotel india
690	foxtrot golf [ROW:0690] hotel india
691	foxtrot golf [ROW:0691] hotel india
692	foxtrot golf [ROW:0692] hotel india
693	foxtrot golf [ROW:0693] hotel india
694	foxtrot golf [ROW:0694] hotel india
695	foxtrot golf [ROW:0695] hotel india
696	foxtrot golf [ROW:0696] hotel india
697	foxtrot golf [ROW:0697] hotel india
698	foxtrot golf [ROW:0698] hotel india
699	foxtrot golf [ROW:0699] hotel india
700	foxtrot golf [ROW:0700] hotel india
701	foxtrot golf [ROW:0701] hotel india
702	foxtrot golf [ROW:0702] hotel india
703	foxtrot golf [ROW:0703] hotel india
704	foxtrot golf [ROW:0704] hotel india
705	foxtrot golf [ROW:0705] hotel india
706	foxtrot golf [ROW:0706] hotel india
707	foxtrot golf [ROW:0707] hotel india
708	foxtrot golf [ROW:0708] hotel india
709	foxtrot golf [ROW:0709] hotel india
710	foxtrot golf [ROW:0710] hotel india
711	foxtrot golf [ROW:0711] hotel india
712	foxtrot golf [ROW:0712] hotel india
713	foxtrot golf [ROW:0713] hotel india
714	foxtrot golf [ROW:0714] hotel india
715	foxtrot golf [ROW:0715] hotel india
716	foxtrot golf [ROW:0716] hotel india
717	foxtrot golf [ROW:0717] hotel india
718	foxtrot golf [ROW:0718] hotel india
719	foxtrot golf [ROW:0719] hotel india
720	foxtrot golf [ROW:0720] hotel india
721	foxtrot golf [ROW:0721] hotel india
722	foxtrot golf [ROW:0722] hotel india
723	foxtrot golf [ROW:0723] hotel india
724	foxtrot golf [ROW:0724] hotel india
725	foxtrot golf [ROW:0725] hotel india
726	foxtrot golf [ROW:0726] hotel india
727	foxtrot golf [ROW:0727] hotel india
728	foxtrot golf [ROW:0728] hotel india
729	foxtrot golf [ROW:0729] hotel india
730	foxtrot golf [ROW:0730] hotel india
731	foxtrot golf [ROW:0731] hotel india
732	foxtrot golf [ROW:0732] hotel india
733	foxtrot golf [ROW:0733] hotel india
734	foxtrot golf [ROW:0734] hotel india
735	foxtrot golf [ROW:0735] hotel india
736	foxtrot golf [ROW:0736] hotel india
737	foxtrot golf [ROW:0737] hotel india
738	foxtrot golf [ROW:0738] hotel india
739	foxtrot golf [ROW:0739] hotel india
740	foxtrot golf [ROW:0740] hotel india
741	foxtrot golf [ROW:0741] hotel india
742	foxtrot golf [ROW:0742] hotel india
743	foxtrot golf [ROW:0743] hotel india
744	foxtrot golf [ROW:0744] hotel india
745	foxtrot golf [ROW:0745] hotel india
746	foxtrot golf [ROW:0746] hotel india
747	foxtrot golf [ROW:0747] hotel india
748	foxtrot golf [ROW:0748] hotel india
749	foxtrot golf [ROW:0749] hotel india
750	foxtrot golf [ROW:0750] hotel india
751	foxtrot golf [ROW:0751] hotel india
752	foxtrot golf [ROW:0752] hotel india
753	foxtrot golf [ROW:0753] hotel india
754	foxtrot golf [ROW:0754] hotel india
755	foxtrot golf [ROW:0755] hotel india
756	foxtrot golf [ROW:0756] hotel india
757	foxtrot golf [ROW:0757] hotel india
758	foxtrot golf [ROW:0758] hotel india
759	foxtrot golf [ROW:0759] hotel india
760	foxtrot golf [ROW:0760] hotel india
761	foxtrot golf [ROW:0761] hotel india
762	foxtrot golf [ROW:0762] hotel india
763	foxtrot golf [ROW:0763] hotel india
764	foxtrot golf [ROW:0764] hotel india
765	foxtrot golf [ROW:0765] hotel india
766	foxtrot golf [ROW:0766] hotel india
767	foxtrot golf [ROW:0767] hotel india
768	foxtrot golf [ROW:0768] hotel india
769	foxtrot golf [ROW:0769] hotel india
770	foxtrot golf [ROW:0770] hotel india
771	foxtrot golf [ROW:0771] hotel india
772	foxtrot golf [ROW:0772] hotel india
773	foxtrot golf [ROW:0773] hotel india
774	foxtrot golf [ROW:0774] hotel india
775	foxtrot golf [ROW:0775] hotel india
776	foxtrot golf [ROW:0776] hotel india
777	foxtrot golf [ROW:0777] hotel india
778	foxtrot golf [ROW:0778] hotel india
779	foxtrot golf [ROW:0779] hotel india
780	foxtrot golf [ROW:0780] hotel india
781	foxtrot golf [ROW:0781] hotel india
782	foxtrot golf [ROW:0782] hotel india
783	foxtrot golf [ROW:0783] hotel india
784	foxtrot golf [ROW:0784] hotel india
785	foxtrot golf [ROW:0785] hotel india
786	foxtrot golf [ROW:0786] hotel india
787	foxtrot golf [ROW:0787] hotel india
788	foxtrot golf [ROW:0788] hotel india
789	foxtrot golf [ROW:0789] hotel india
790	foxtrot golf [ROW:0790] hotel india
791	foxtrot golf [ROW:0791] hotel india
792	foxtrot golf [ROW:0792] hotel india
793	foxtrot golf [ROW:0793] hotel india
794	foxtrot golf [ROW:0794] hotel india
795	foxtrot golf [ROW:0795] hotel india
796	foxtrot golf [ROW:0796] hotel india
797	foxtrot golf [ROW:0797] hotel india
798	foxtrot golf [ROW:0798] hotel india
799	foxtrot golf [ROW:0799] hotel india
800	foxtrot golf [ROW:0800] hotel india
801	foxtrot golf [ROW:0801] hotel india
802	foxtrot golf [ROW:0802] hotel india
803	foxtrot golf [ROW:0803] hotel india
804	foxtrot golf [ROW:0804] hotel india
805	foxtrot golf [ROW:0805] hotel india
806	foxtrot golf [ROW:0806] hotel india
807	foxtrot golf [ROW:0807] hotel india
808	foxtrot golf [ROW:0808] hotel india
809	foxtrot golf [ROW:0809] hotel india
810	foxtrot golf [ROW:0810] hotel india
811	foxtrot golf [ROW:0811] hotel india
812	foxtrot golf [ROW:0812] hotel india
813	foxtrot golf [ROW:0813] hotel india
814	foxtrot golf [ROW:0814] hotel india
815	foxtrot golf [ROW:0815] hotel india
816	foxtrot golf [ROW:0816] hotel india
817	foxtrot golf [ROW:0817] hotel india
818	foxtrot golf [ROW:0818] hotel india
819	foxtrot golf [ROW:0819] hotel india
820	foxtrot golf [ROW:0820] hotel india
821	foxtrot golf [ROW:0821] hotel india
822	foxtrot golf [ROW:0822] hotel india
823	foxtrot golf [ROW:0823] hotel india
824	foxtrot golf [ROW:0824] hotel india
825	foxtrot golf [ROW:0825] hotel india
826	foxtrot golf [ROW:0826] hotel india
827	foxtrot golf [ROW:0827] hotel india
828	foxtrot golf [ROW:0828] hotel india
829	foxtrot golf [ROW:0829] hotel india
830	foxtrot golf [ROW:0830] hotel india
831	foxtrot golf [ROW:0831] hotel india
832	foxtrot golf [ROW:0832] hotel india
833	foxtrot golf [ROW:0833] hotel india
834	foxtrot golf [ROW:0834] hotel india
835	foxtrot golf [ROW:0835] hotel india
836	foxtrot golf [ROW:0836] hotel india
837	foxtrot golf [ROW:0837] hotel india
838	foxtrot golf [ROW:0838] hotel india
839	foxtrot golf [ROW:0839] hotel india
840	foxtrot golf [ROW:0840] hotel india
841	foxtrot golf [ROW:0841] hotel india
842	foxtrot golf [ROW:0842] hotel india
843	foxtrot golf [ROW:0843] hotel india
844	foxtrot golf [ROW:0844] hotel india
845	foxtrot golf [ROW:0845] hotel india
846	foxtrot golf [ROW:0846] hotel india
847	foxtrot golf [ROW:0847] hotel india
848	foxtrot golf [ROW:0848] hotel india
849	foxtrot golf [ROW:0849] hotel india
850	foxtrot golf [ROW:0850] hotel india
851	foxtrot golf [ROW:0851] hotel india
852	foxtrot golf [ROW:0852] hotel india
853	foxtrot golf [ROW:0853] hotel india
854	foxtrot golf [ROW:0854] hotel india
855	foxtrot golf [ROW:0855] hotel india
856	foxtrot golf [ROW:0856] hotel india
857	foxtrot golf [ROW:0857] hotel india
858	foxtrot golf [ROW:0858] hotel india
859	foxtrot golf [ROW:0859] hotel india
860	foxtrot golf [ROW:0860] hotel india
861	foxtrot golf [ROW:0861] hotel india
862	foxtrot golf [ROW:0862] hotel india
863	foxtrot golf [ROW:0863] hotel india
864	foxtrot golf [ROW:0864] hotel india
865	foxtrot golf [ROW:0865] hotel india
866	foxtrot golf [ROW:0866] hotel india
867	foxtrot golf [ROW:0867] hotel india
868	foxtrot golf [ROW:0868] hotel india
869	foxtrot golf [ROW:0869] hotel india
870	foxtrot golf [ROW:0870] hotel india
871	foxtrot golf [ROW:0871] hotel india
872	foxtrot golf [ROW:0872] hotel india
873	foxtrot golf [ROW:0873] hotel india
874	foxtrot golf [ROW:0874] hotel india
875	foxtrot golf [ROW:0875] hotel india
876	foxtrot golf [ROW:0876] hotel india
877	foxtrot golf [ROW:0877] hotel india
878	foxtrot golf [ROW:0878] hotel india
879	foxtrot golf [ROW:0879] hotel india
880	foxtrot golf [ROW:0880] hotel india
881	foxtrot golf [ROW:0881] hotel india
882	foxtrot golf [ROW:0882] hotel india
883	foxtrot golf [ROW:0883] hotel india
884	foxtrot golf [ROW:0884] hotel india
885	foxtrot golf [ROW:0885] hotel india
886	foxtrot golf [ROW:0886] hotel india
887	foxtrot golf [ROW:0887] hotel india
888	foxtrot golf [ROW:0888] hotel india
889	foxtrot golf [ROW:0889] hotel india
890	foxtrot golf [ROW:0890] hotel india
891	foxtrot golf [ROW:0891] hotel india
892	foxtrot golf [ROW:0892] hotel india
893	foxtrot golf [ROW:0893] hotel india
894	foxtrot golf [ROW:0894] hotel india
895	foxtrot golf [ROW:0895] hotel india
896	foxtrot golf [ROW:0896] hotel india
897	foxtrot golf [ROW:0897] hotel india
898	foxtrot golf [ROW:0898] hotel india
899	foxtrot golf [ROW:0899] hotel india
900	foxtrot golf [ROW:0900] hotel india
901	foxtrot golf [ROW:0901] hotel india
902	foxtrot golf [ROW:0902] hotel india
903	foxtrot golf [ROW:0903] hotel india
904	foxtrot golf [ROW:0904] hotel india
905	foxtrot golf [ROW:0905] hotel india
906	foxtrot golf [ROW:0906] hotel india
907	foxtrot golf [ROW:0907] hotel india
908	foxtrot golf [ROW:0908] hotel india
909	foxtrot golf [ROW:0909] hotel india
910	foxtrot golf [ROW:0910] hotel india
911	foxtrot golf [ROW:0911] hotel india
912	foxtrot golf [ROW:0912] hotel india
913	foxtrot golf [ROW:0913] hotel india
914	foxtrot golf [ROW:0914] hotel india
915	foxtrot golf [ROW:0915] hotel india
916	foxtrot golf [ROW:0916] hotel india
917	foxtrot golf [ROW:0917] hotel india
918	foxtrot golf [ROW:0918] hotel india
919	foxtrot golf [ROW:0919] hotel india
920	foxtrot golf [ROW:0920] hotel india
921	foxtrot golf [ROW:0921] hotel india
922	foxtrot golf [ROW:0922] hotel india
923	foxtrot golf [ROW:0923] hotel india
924	foxtrot golf [ROW:0924] hotel india
925	foxtrot golf [ROW:0925] hotel india
926	foxtrot golf [ROW:0926] hotel india
927	foxtrot golf [ROW:0927] hotel india
928	foxtrot golf [ROW:0928] hotel india
929	foxtrot golf [ROW:0929] hotel india
930	foxtrot golf [ROW:0930] hotel india
931	foxtrot golf [ROW:0931] hotel india
932	foxtrot golf [ROW:0932] hotel india
933	foxtrot golf [ROW:0933] hotel india
934	foxtrot golf [ROW:0934] hotel india
935	foxtrot golf [ROW:0935] hotel india
936	foxtrot golf [ROW:0936] hotel india
937	foxtrot golf [ROW:0937] hotel india
938	foxtrot golf [ROW:0938] hotel india
939	foxtrot golf [ROW:0939] hotel india
940	foxtrot golf [ROW:0940] hotel india
941	foxtrot golf [ROW:0941] hotel india
942	foxtrot golf [ROW:0942] hotel india
943	foxtrot golf [ROW:0943] hotel india
944	foxtrot golf [ROW:0944] hotel india
945	foxtrot golf [ROW:0945] hotel india
946	foxtrot golf [ROW:0946] hotel india
947	foxtrot golf [ROW:0947] hotel india
948	foxtrot golf [ROW:0948] hotel india
949	foxtrot golf [ROW:0949] hotel india
950	foxtrot golf [ROW:0950] hotel india
951	foxtrot golf [ROW:0951] hotel india
952	foxtrot golf [ROW:0952] hotel india
953	foxtrot golf [ROW:0953] hotel india
954	foxtrot golf [ROW:0954] hotel india
955	foxtrot golf [ROW:0955] hotel india
956	foxtrot golf [ROW:0956] hotel india
957	foxtrot golf [ROW:0957] hotel india
958	foxtrot golf [ROW:0958] hotel india
959	foxtrot golf [ROW:0959] hotel india
960	foxtrot golf [ROW:0960] hotel india
961	foxtrot golf [ROW:0961] hotel india
962	foxtrot golf [ROW:0962] hotel india
963	foxtrot golf [ROW:0963] hotel india
964	foxtrot golf [ROW:0964] hotel india
965	foxtrot golf [ROW:0965] hotel india
966	foxtrot golf [ROW:0966] hotel india
967	foxtrot golf [ROW:0967] hotel india
968	foxtrot golf [ROW:0968] hotel india
969	foxtrot golf [ROW:0969] hotel india
970	foxtrot golf [ROW:0970] hotel india
971	foxtrot golf [ROW:0971] hotel india
972	foxtrot golf [ROW:0972] hotel india
973	foxtrot golf [ROW:0973] hotel india
974	foxtrot golf [ROW:0974] hotel india
975	foxtrot golf [ROW:0975] hotel india
976	foxtrot golf [ROW:0976] hotel india
977	foxtrot golf [ROW:0977] hotel india
978	foxtrot golf [ROW:0978] hotel india
979	foxtrot golf [ROW:0979] hotel india
980	foxtrot golf [ROW:0980] hotel india
981	foxtrot golf [ROW:0981] hotel india
982	foxtrot golf [ROW:0982] hotel india
983	foxtrot golf [ROW:0983] hotel india
984	foxtrot golf [ROW:0984] hotel india
985	foxtrot golf [ROW:0985] hotel india
986	foxtrot golf [ROW:0986] hotel india
987	foxtrot golf [ROW:0987] hotel india
988	foxtrot golf [ROW:0988] hotel india
989	foxtrot golf [ROW:0989] hotel india
990	foxtrot golf [ROW:0990] hotel india
991	foxtrot golf [ROW:0991] hotel india
992	foxtrot golf [ROW:0992] hotel india
993	foxtrot golf [ROW:0993] hotel india
994	foxtrot golf [ROW:0994] hotel india
995	foxtrot golf [ROW:0995] hotel india
996	foxtrot golf [ROW:0996] hotel india
997	foxtrot golf [ROW:0997] hotel india
998	foxtrot golf [ROW:0998] hotel india
999	foxtrot golf [ROW:0999] hotel india
1000	foxtrot golf [ROW:1000] hotel india
1001	foxtrot golf [ROW:1001] hotel india
1002	foxtrot golf [ROW:1002] hotel india
1003	foxtrot golf [ROW:1003] hotel india
1004	foxtrot golf [ROW:1004] hotel india
1005	foxtrot golf [ROW:1005] hotel india
1006	foxtrot golf [ROW:1006] hotel india
1007	foxtrot golf [ROW:1007] hotel india
1008	foxtrot golf [ROW:1008] hotel india
1009	foxtrot golf [ROW:1009] hotel india
1010	foxtrot golf [ROW:1010] hotel india
1011	foxtrot golf [ROW:1011] hotel india
1012	foxtrot golf [ROW:1012] hotel india
1013	foxtrot golf [ROW:1013] hotel india
1014	foxtrot golf [ROW:1014] hotel india
1015	foxtrot golf [ROW:1015] hotel india
1016	foxtrot golf [ROW:1016] hotel india
1017	foxtrot golf [ROW:1017] hotel india
1018	foxtrot golf [ROW:1018] hotel india
1019	foxtrot golf [ROW:1019] hotel india
1020	foxtrot golf [ROW:1020] hotel india
1021	foxtrot golf [ROW:1021] hotel india
1022	foxtrot golf [ROW:1022] hotel india
1023	foxtrot golf [ROW:1023] hotel india
1024	foxtrot golf [ROW:1024] hotel india
1025	foxtrot golf [ROW:1025] hotel india
1026	foxtrot golf [ROW:1026] hotel india
1027	foxtrot golf [ROW:1027] hotel india
1028	foxtrot golf [ROW:1028] hotel india
1029	foxtrot golf [ROW:1029] hotel india
1030	foxtrot golf [ROW:1030] hotel india
1031	foxtrot golf [ROW:1031] hotel india
1032	foxtrot golf [ROW:1032] hotel india
1033	foxtrot golf [ROW:1033] hotel india
1034	foxtrot golf [ROW:1034] hotel india
1035	foxtrot golf [ROW:1035] hotel india
1036	foxtrot golf [ROW:1036] hotel india
1037	foxtrot golf [ROW:1037] hotel india
1038	foxtrot golf [ROW:1038] hotel india
1039	foxtrot golf [ROW:1039] hotel india
1040	foxtrot golf [ROW:1040] hotel india
1041	foxtrot golf [ROW:1041] hotel india
1042	foxtrot golf [ROW:1042] hotel india
1043	foxtrot golf [ROW:1043] hotel india
1044	foxtrot golf [ROW:1044] hotel india
1045	foxtrot golf [ROW:1045] hotel india
1046	foxtrot golf [ROW:1046] hotel india
1047	foxtrot golf [ROW:1047] hotel india
1048	foxtrot golf [ROW:1048] hotel india
1049	foxtrot golf [ROW:1049] hotel india
1050	foxtrot golf [ROW:1050] hotel india
1051	foxtrot golf [ROW:1051] hotel india
1052	foxtrot golf [ROW:1052] hotel india
1053	foxtrot golf [ROW:1053] hotel india
1054	foxtrot golf [ROW:1054] hotel india
1055	foxtrot golf [ROW:1055] hotel india
1056	foxtrot golf [ROW:1056] hotel india
1057	foxtrot golf [ROW:1057] hotel india
1058	foxtrot golf [ROW:1058] hotel india
1059	foxtrot golf [ROW:1059] hotel india
1060	foxtrot golf [ROW:1060] hotel india
1061	foxtrot golf [ROW:1061] hotel india
1062	foxtrot golf [ROW:1062] hotel india
--- RAW OUTPUT END ---

The following block was delivered immediately after the tool result above, outside the [result] element, as a separate system-injected block:

--- RAW OUTPUT START ---
[Truncated: PARTIAL view — C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\large-file-3000-lines.txt: showing lines 1-1062 of 3001 total (60007 tokens, cap 25000). Call Read with offset=1063 limit=1062 for the next page, or Grep to find a specific section. To read it anyway, up to what still fits in your context, call Read again with allow_large: true — only if you genuinely need all of it or the user asked for the whole file. Do NOT answer from this page alone if the answer may be further in the file.]
--- RAW OUTPUT END ---

## Glob — With matches

**Call:** `Glob({pattern: "*.txt", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts"})`

--- RAW OUTPUT START ---
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt
HarnessKnowledge\SystemPromptCapture\TestArtifacts\large-file-3000-lines.txt
--- RAW OUTPUT END ---

## Glob — No matches

**Call:** `Glob({pattern: "*.nomatchext", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts"})`

--- RAW OUTPUT START ---
No files found
--- RAW OUTPUT END ---

## Grep — With matches (default output_mode files_with_matches)

**Call:** `Grep({pattern: "DISTINCT_MATCH_TOKEN_ABC", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts"})`

--- RAW OUTPUT START ---
Found 1 file
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt
--- RAW OUTPUT END ---

## Grep — With matches (output_mode content)

**Call:** `Grep({pattern: "DISTINCT_MATCH_TOKEN_ABC", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts", output_mode: "content"})`

--- RAW OUTPUT START ---
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:19:Line 19: Grep target: DISTINCT_MATCH_TOKEN_ABC
--- RAW OUTPUT END ---

## Grep — No matches

**Call:** `Grep({pattern: "NO_SUCH_TOKEN_XYZ_987", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts"})`

--- RAW OUTPUT START ---
No files found
--- RAW OUTPUT END ---

## Grep — With include filter (glob), output_mode content

**Call:** `Grep({pattern: "Line 1\d", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts", glob: "varied-*.txt", output_mode: "content"})`

--- RAW OUTPUT START ---
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:10:Line 10: Path-like content: /opt/tools/bin/run --mode=capture
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:11:Line 11: Code-like content: const calc = (x) => { return x > 0 && x < 100; }
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:12:Line 12: Regex-like content: ^begin.*center[a-z]+finish$
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:13:Line 13: XML-like content: <node role="test">nested value</node>
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:14:Line 14: JSON-like content: {"name": "probe", "count": 77, "items": [4, 5, 6]}
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:16:Line 16: Swap this line with different content.
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:18:Line 18: This line must stay intact after edit tests.
HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:19:Line 19: Grep target: DISTINCT_MATCH_TOKEN_ABC
--- RAW OUTPUT END ---

## Grep — output_mode count

**Call:** `Grep({pattern: "ROW:00[0-2]\d", path: "C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts", output_mode: "count"})`

--- RAW OUTPUT START ---
HarnessKnowledge\SystemPromptCapture\TestArtifacts\large-file-3000-lines.txt:29

Found 29 total occurrences across 1 file.
--- RAW OUTPUT END ---

## Edit — Success (temp copy of varied-content-test.txt, line between EDIT TARGET markers)

**Call:** `Edit({file_path: "C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\edit-test-copy.txt", old_string: "Line 16: Swap this line with different content.", new_string: "Line 16: EDITED by tool output capture."})`

--- RAW OUTPUT START ---
The file C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\edit-test-copy.txt has been updated successfully.
--- RAW OUTPUT END ---

## Edit — Error: old_string not found

**Call:** `Edit({file_path: "C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\edit-test-copy.txt", old_string: "This string does not exist in the file XYZ", new_string: "irrelevant"})`

--- RAW OUTPUT START ---
[error][tool_use_error]String to replace not found in file.
String: This string does not exist in the file XYZ[/tool_use_error][/error]
--- RAW OUTPUT END ---

## Edit — Error: multiple matches without replace_all

**Call:** `Edit({file_path: "C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\edit-test-copy.txt", old_string: "--- EDIT TARGET", new_string: "--- EDIT MARKER"})`

--- RAW OUTPUT START ---
[error][tool_use_error]Found 2 matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.
String: --- EDIT TARGET[/tool_use_error][/error]
--- RAW OUTPUT END ---

## Write — Success (new file)

**Call:** `Write({file_path: "C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\write-test-new.txt", content: "Write tool output capture test.\nSecond line.\n"})`

--- RAW OUTPUT START ---
File created successfully at: C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\write-test-new.txt (file state is current in your context — no need to Read it back)
--- RAW OUTPUT END ---

## Write — Success (overwrite existing file previously edited in session)

**Call:** `Write({file_path: "C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\edit-test-copy.txt", content: "Overwrite test.\n"})`

--- RAW OUTPUT START ---
The file C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\edit-test-copy.txt has been updated successfully. (file state is current in your context — no need to Read it back)
--- RAW OUTPUT END ---

## Write — Overwrite existing file never Read in session (created via Bash)

**Call:** `Write({file_path: "C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\unread-existing.txt", content: "overwrite attempt\n"})`

--- RAW OUTPUT START ---
The file C:\Users\tgurt\AppData\Local\Temp\claude\1aa80f994f2a6e0b\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\scratchpad\unread-existing.txt has been updated successfully. (file state is current in your context — no need to Read it back)
--- RAW OUTPUT END ---

## Bash — Success

**Call:** `Bash({command: "echo \"hello from bash\"; pwd", description: "Print a test string and working directory"})`

--- RAW OUTPUT START ---
hello from bash
/c/AI/MOSAIC/MOSAIC
--- RAW OUTPUT END ---

## Bash — Success with no output

**Call:** `Bash({command: "true", description: "Run a command with no output"})`

--- RAW OUTPUT START ---
(Bash completed with no output)
--- RAW OUTPUT END ---

## Bash — Error: nonzero exit with stdout and stderr

**Call:** `Bash({command: "echo \"to stdout\"; echo \"to stderr\" >&2; exit 3", description: "Emit stdout and stderr then exit with code 3"})`

--- RAW OUTPUT START ---
[error]Exit code 3
to stdout
to stderr[/error]
--- RAW OUTPUT END ---

## Bash — Error: command failure (ls on missing directory)

**Call:** `Bash({command: "ls /nonexistent-directory-xyz", description: "List a missing directory to produce an error"})`

--- RAW OUTPUT START ---
[error]Exit code 2
ls: cannot access '/nonexistent-directory-xyz': No such file or directory[/error]
--- RAW OUTPUT END ---

## Bash — Long output (seq 1 5000)

**Call:** `Bash({command: "seq 1 5000", description: "Print numbers 1 to 5000 to test long output"})`

--- RAW OUTPUT START ---
1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
51
52
53
54
55
56
57
58
59
60
61
62
63
64
65
66
67
68
69
70
71
72
73
74
75
76
77
78
79
80
81
82
83
84
85
86
87
88
89
90
91
92
93
94
95
96
97
98
99
100
101
102
103
104
105
106
107
108
109
110
111
112
113
114
115
116
117
118
119
120
121
122
123
124
125
126
127
128
129
130
131
132
133
134
135
136
137
138
139
140
141
142
143
144
145
146
147
148
149
150
151
152
153
154
155
156
157
158
159
160
161
162
163
164
165
166
167
168
169
170
171
172
173
174
175
176
177
178
179
180
181
182
183
184
185
186
187
188
189
190
191
192
193
194
195
196
197
198
199
200
201
202
203
204
205
206
207
208
209
210
211
212
213
214
215
216
217
218
219
220
221
222
223
224
225
226
227
228
229
230
231
232
233
234
235
236
237
238
239
240
241
242
243
244
245
246
247
248
249
250
251
252
253
254
255
256
257
258
259
260
261
262
263
264
265
266
267
268
269
270
271
272
273
274
275
276
277
278
279
280
281
282
283
284
285
286
287
288
289
290
291
292
293
294
295
296
297
298
299
300
301
302
303
304
305
306
307
308
309
310
311
312
313
314
315
316
317
318
319
320
321
322
323
324
325
326
327
328
329
330
331
332
333
334
335
336
337
338
339
340
341
342
343
344
345
346
347
348
349
350
351
352
353
354
355
356
357
358
359
360
361
362
363
364
365
366
367
368
369
370
371
372
373
374
375
376
377
378
379
380
381
382
383
384
385
386
387
388
389
390
391
392
393
394
395
396
397
398
399
400
401
402
403
404
405
406
407
408
409
410
411
412
413
414
415
416
417
418
419
420
421
422
423
424
425
426
427
428
429
430
431
432
433
434
435
436
437
438
439
440
441
442
443
444
445
446
447
448
449
450
451
452
453
454
455
456
457
458
459
460
461
462
463
464
465
466
467
468
469
470
471
472
473
474
475
476
477
478
479
480
481
482
483
484
485
486
487
488
489
490
491
492
493
494
495
496
497
498
499
500
501
502
503
504
505
506
507
508
509
510
511
512
513
514
515
516
517
518
519
520
521
522
523
524
525
526
527
528
529
530
531
532
533
534
535
536
537
538
539
540
541
542
543
544
545
546
547
548
549
550
551
552
553
554
555
556
557
558
559
560
561
562
563
564
565
566
567
568
569
570
571
572
573
574
575
576
577
578
579
580
581
582
583
584
585
586
587
588
589
590
591
592
593
594
595
596
597
598
599
600
601
602
603
604
605
606
607
608
609
610
611
612
613
614
615
616
617
618
619
620
621
622
623
624
625
626
627
628
629
630
631
632
633
634
635
636
637
638
639
640
641
642
643
644
645
646
647
648
649
650
651
652
653
654
655
656
657
658
659
660
661
662
663
664
665
666
667
668
669
670
671
672
673
674
675
676
677
678
679
680
681
682
683
684
685
686
687
688
689
690
691
692
693
694
695
696
697
698
699
700
701
702
703
704
705
706
707
708
709
710
711
712
713
714
715
716
717
718
719
720
721
722
723
724
725
726
727
728
729
730
731
732
733
734
735
736
737
738
739
740
741
742
743
744
745
746
747
748
749
750
751
752
753
754
755
756
757
758
759
760
761
762
763
764
765
766
767
768
769
770
771
772
773
774
775
776
777
778
779
780
781
782
783
784
785
786
787
788
789
790
791
792
793
794
795
796
797
798
799
800
801
802
803
804
805
806
807
808
809
810
811
812
813
814
815
816
817
818
819
820
821
822
823
824
825
826
827
828
829
830
831
832
833
834
835
836
837
838
839
840
841
842
843
844
845
846
847
848
849
850
851
852
853
854
855
856
857
858
859
860
861
862
863
864
865
866
867
868
869
870
871
872
873
874
875
876
877
878
879
880
881
882
883
884
885
886
887
888
889
890
891
892
893
894
895
896
897
898
899
900
901
902
903
904
905
906
907
908
909
910
911
912
913
914
915
916
917
918
919
920
921
922
923
924
925
926
927
928
929
930
931
932
933
934
935
936
937
938
939
940
941
942
943
944
945
946
947
948
949
950
951
952
953
954
955
956
957
958
959
960
961
962
963
964
965
966
967
968
969
970
971
972
973
974
975
976
977
978
979
980
981
982
983
984
985
986
987
988
989
990
991
992
993
994
995
996
997
998
999
1000
1001
1002
1003
1004
1005
1006
1007
1008
1009
1010
1011
1012
1013
1014
1015
1016
1017
1018
1019
1020
1021
1022
1023
1024
1025
1026
1027
1028
1029
1030
1031
1032
1033
1034
1035
1036
1037
1038
1039
1040
1041
1042
1043
1044
1045
1046
1047
1048
1049
1050
1051
1052
1053
1054
1055
1056
1057
1058
1059
1060
1061
1062
1063
1064
1065
1066
1067
1068
1069
1070
1071
1072
1073
1074
1075
1076
1077
1078
1079
1080
1081
1082
1083
1084
1085
1086
1087
1088
1089
1090
1091
1092
1093
1094
1095
1096
1097
1098
1099
1100
1101
1102
1103
1104
1105
1106
1107
1108
1109
1110
1111
1112
1113
1114
1115
1116
1117
1118
1119
1120
1121
1122
1123
1124
1125
1126
1127
1128
1129
1130
1131
1132
1133
1134
1135
1136
1137
1138
1139
1140
1141
1142
1143
1144
1145
1146
1147
1148
1149
1150
1151
1152
1153
1154
1155
1156
1157
1158
1159
1160
1161
1162
1163
1164
1165
1166
1167
1168
1169
1170
1171
1172
1173
1174
1175
1176
1177
1178
1179
1180
1181
1182
1183
1184
1185
1186
1187
1188
1189
1190
1191
1192
1193
1194
1195
1196
1197
1198
1199
1200
1201
1202
1203
1204
1205
1206
1207
1208
1209
1210
1211
1212
1213
1214
1215
1216
1217
1218
1219
1220
1221
1222
1223
1224
1225
1226
1227
1228
1229
1230
1231
1232
1233
1234
1235
1236
1237
1238
1239
1240
1241
1242
1243
1244
1245
1246
1247
1248
1249
1250
1251
1252
1253
1254
1255
1256
1257
1258
1259
1260
1261
1262
1263
1264
1265
1266
1267
1268
1269
1270
1271
1272
1273
1274
1275
1276
1277
1278
1279
1280
1281
1282
1283
1284
1285
1286
1287
1288
1289
1290
1291
1292
1293
1294
1295
1296
1297
1298
1299
1300
1301
1302
1303
1304
1305
1306
1307
1308
1309
1310
1311
1312
1313
1314
1315
1316
1317
1318
1319
1320
1321
1322
1323
1324
1325
1326
1327
1328
1329
1330
1331
1332
1333
1334
1335
1336
1337
1338
1339
1340
1341
1342
1343
1344
1345
1346
1347
1348
1349
1350
1351
1352
1353
1354
1355
1356
1357
1358
1359
1360
1361
1362
1363
1364
1365
1366
1367
1368
1369
1370
1371
1372
1373
1374
1375
1376
1377
1378
1379
1380
1381
1382
1383
1384
1385
1386
1387
1388
1389
1390
1391
1392
1393
1394
1395
1396
1397
1398
1399
1400
1401
1402
1403
1404
1405
1406
1407
1408
1409
1410
1411
1412
1413
1414
1415
1416
1417
1418
1419
1420
1421
1422
1423
1424
1425
1426
1427
1428
1429
1430
1431
1432
1433
1434
1435
1436
1437
1438
1439
1440
1441
1442
1443
1444
1445
1446
1447
1448
1449
1450
1451
1452
1453
1454
1455
1456
1457
1458
1459
1460
1461
1462
1463
1464
1465
1466
1467
1468
1469
1470
1471
1472
1473
1474
1475
1476
1477
1478
1479
1480
1481
1482
1483
1484
1485
1486
1487
1488
1489
1490
1491
1492
1493
1494
1495
1496
1497
1498
1499
1500
1501
1502
1503
1504
1505
1506
1507
1508
1509
1510
1511
1512
1513
1514
1515
1516
1517
1518
1519
1520
1521
1522
1523
1524
1525
1526
1527
1528
1529
1530
1531
1532
1533
1534
1535
1536
1537
1538
1539
1540
1541
1542
1543
1544
1545
1546
1547
1548
1549
1550
1551
1552
1553
1554
1555
1556
1557
1558
1559
1560
1561
1562
1563
1564
1565
1566
1567
1568
1569
1570
1571
1572
1573
1574
1575
1576
1577
1578
1579
1580
1581
1582
1583
1584
1585
1586
1587
1588
1589
1590
1591
1592
1593
1594
1595
1596
1597
1598
1599
1600
1601
1602
1603
1604
1605
1606
1607
1608
1609
1610
1611
1612
1613
1614
1615
1616
1617
1618
1619
1620
1621
1622
1623
1624
1625
1626
1627
1628
1629
1630
1631
1632
1633
1634
1635
1636
1637
1638
1639
1640
1641
1642
1643
1644
1645
1646
1647
1648
1649
1650
1651
1652
1653
1654
1655
1656
1657
1658
1659
1660
1661
1662
1663
1664
1665
1666
1667
1668
1669
1670
1671
1672
1673
1674
1675
1676
1677
1678
1679
1680
1681
1682
1683
1684
1685
1686
1687
1688
1689
1690
1691
1692
1693
1694
1695
1696
1697
1698
1699
1700
1701
1702
1703
1704
1705
1706
1707
1708
1709
1710
1711
1712
1713
1714
1715
1716
1717
1718
1719
1720
1721
1722
1723
1724
1725
1726
1727
1728
1729
1730
1731
1732
1733
1734
1735
1736
1737
1738
1739
1740
1741
1742
1743
1744
1745
1746
1747
1748
1749
1750
1751
1752
1753
1754
1755
1756
1757
1758
1759
1760
1761
1762
1763
1764
1765
1766
1767
1768
1769
1770
1771
1772
1773
1774
1775
1776
1777
1778
1779
1780
1781
1782
1783
1784
1785
1786
1787
1788
1789
1790
1791
1792
1793
1794
1795
1796
1797
1798
1799
1800
1801
1802
1803
1804
1805
1806
1807
1808
1809
1810
1811
1812
1813
1814
1815
1816
1817
1818
1819
1820
1821
1822
1823
1824
1825
1826
1827
1828
1829
1830
1831
1832
1833
1834
1835
1836
1837
1838
1839
1840
1841
1842
1843
1844
1845
1846
1847
1848
1849
1850
1851
1852
1853
1854
1855
1856
1857
1858
1859
1860
1861
1862
1863
1864
1865
1866
1867
1868
1869
1870
1871
1872
1873
1874
1875
1876
1877
1878
1879
1880
1881
1882
1883
1884
1885
1886
1887
1888
1889
1890
1891
1892
1893
1894
1895
1896
1897
1898
1899
1900
1901
1902
1903
1904
1905
1906
1907
1908
1909
1910
1911
1912
1913
1914
1915
1916
1917
1918
1919
1920
1921
1922
1923
1924
1925
1926
1927
1928
1929
1930
1931
1932
1933
1934
1935
1936
1937
1938
1939
1940
1941
1942
1943
1944
1945
1946
1947
1948
1949
1950
1951
1952
1953
1954
1955
1956
1957
1958
1959
1960
1961
1962
1963
1964
1965
1966
1967
1968
1969
1970
1971
1972
1973
1974
1975
1976
1977
1978
1979
1980
1981
1982
1983
1984
1985
1986
1987
1988
1989
1990
1991
1992
1993
1994
1995
1996
1997
1998
1999
2000
2001
2002
2003
2004
2005
2006
2007
2008
2009
2010
2011
2012
2013
2014
2015
2016
2017
2018
2019
2020
2021
2022
2023
2024
2025
2026
2027
2028
2029
2030
2031
2032
2033
2034
2035
2036
2037
2038
2039
2040
2041
2042
2043
2044
2045
2046
2047
2048
2049
2050
2051
2052
2053
2054
2055
2056
2057
2058
2059
2060
2061
2062
2063
2064
2065
2066
2067
2068
2069
2070
2071
2072
2073
2074
2075
2076
2077
2078
2079
2080
2081
2082
2083
2084
2085
2086
2087
2088
2089
2090
2091
2092
2093
2094
2095
2096
2097
2098
2099
2100
2101
2102
2103
2104
2105
2106
2107
2108
2109
2110
2111
2112
2113
2114
2115
2116
2117
2118
2119
2120
2121
2122
2123
2124
2125
2126
2127
2128
2129
2130
2131
2132
2133
2134
2135
2136
2137
2138
2139
2140
2141
2142
2143
2144
2145
2146
2147
2148
2149
2150
2151
2152
2153
2154
2155
2156
2157
2158
2159
2160
2161
2162
2163
2164
2165
2166
2167
2168
2169
2170
2171
2172
2173
2174
2175
2176
2177
2178
2179
2180
2181
2182
2183
2184
2185
2186
2187
2188
2189
2190
2191
2192
2193
2194
2195
2196
2197
2198
2199
2200
2201
2202
2203
2204
2205
2206
2207
2208
2209
2210
2211
2212
2213
2214
2215
2216
2217
2218
2219
2220
2221
2222
2223
2224
2225
2226
2227
2228
2229
2230
2231
2232
2233
2234
2235
2236
2237
2238
2239
2240
2241
2242
2243
2244
2245
2246
2247
2248
2249
2250
2251
2252
2253
2254
2255
2256
2257
2258
2259
2260
2261
2262
2263
2264
2265
2266
2267
2268
2269
2270
2271
2272
2273
2274
2275
2276
2277
2278
2279
2280
2281
2282
2283
2284
2285
2286
2287
2288
2289
2290
2291
2292
2293
2294
2295
2296
2297
2298
2299
2300
2301
2302
2303
2304
2305
2306
2307
2308
2309
2310
2311
2312
2313
2314
2315
2316
2317
2318
2319
2320
2321
2322
2323
2324
2325
2326
2327
2328
2329
2330
2331
2332
2333
2334
2335
2336
2337
2338
2339
2340
2341
2342
2343
2344
2345
2346
2347
2348
2349
2350
2351
2352
2353
2354
2355
2356
2357
2358
2359
2360
2361
2362
2363
2364
2365
2366
2367
2368
2369
2370
2371
2372
2373
2374
2375
2376
2377
2378
2379
2380
2381
2382
2383
2384
2385
2386
2387
2388
2389
2390
2391
2392
2393
2394
2395
2396
2397
2398
2399
2400
2401
2402
2403
2404
2405
2406
2407
2408
2409
2410
2411
2412
2413
2414
2415
2416
2417
2418
2419
2420
2421
2422
2423
2424
2425
2426
2427
2428
2429
2430
2431
2432
2433
2434
2435
2436
2437
2438
2439
2440
2441
2442
2443
2444
2445
2446
2447
2448
2449
2450
2451
2452
2453
2454
2455
2456
2457
2458
2459
2460
2461
2462
2463
2464
2465
2466
2467
2468
2469
2470
2471
2472
2473
2474
2475
2476
2477
2478
2479
2480
2481
2482
2483
2484
2485
2486
2487
2488
2489
2490
2491
2492
2493
2494
2495
2496
2497
2498
2499
2500
2501
2502
2503
2504
2505
2506
2507
2508
2509
2510
2511
2512
2513
2514
2515
2516
2517
2518
2519
2520
2521
2522
2523
2524
2525
2526
2527
2528
2529
2530
2531
2532
2533
2534
2535
2536
2537
2538
2539
2540
2541
2542
2543
2544
2545
2546
2547
2548
2549
2550
2551
2552
2553
2554
2555
2556
2557
2558
2559
2560
2561
2562
2563
2564
2565
2566
2567
2568
2569
2570
2571
2572
2573
2574
2575
2576
2577
2578
2579
2580
2581
2582
2583
2584
2585
2586
2587
2588
2589
2590
2591
2592
2593
2594
2595
2596
2597
2598
2599
2600
2601
2602
2603
2604
2605
2606
2607
2608
2609
2610
2611
2612
2613
2614
2615
2616
2617
2618
2619
2620
2621
2622
2623
2624
2625
2626
2627
2628
2629
2630
2631
2632
2633
2634
2635
2636
2637
2638
2639
2640
2641
2642
2643
2644
2645
2646
2647
2648
2649
2650
2651
2652
2653
2654
2655
2656
2657
2658
2659
2660
2661
2662
2663
2664
2665
2666
2667
2668
2669
2670
2671
2672
2673
2674
2675
2676
2677
2678
2679
2680
2681
2682
2683
2684
2685
2686
2687
2688
2689
2690
2691
2692
2693
2694
2695
2696
2697
2698
2699
2700
2701
2702
2703
2704
2705
2706
2707
2708
2709
2710
2711
2712
2713
2714
2715
2716
2717
2718
2719
2720
2721
2722
2723
2724
2725
2726
2727
2728
2729
2730
2731
2732
2733
2734
2735
2736
2737
2738
2739
2740
2741
2742
2743
2744
2745
2746
2747
2748
2749
2750
2751
2752
2753
2754
2755
2756
2757
2758
2759
2760
2761
2762
2763
2764
2765
2766
2767
2768
2769
2770
2771
2772
2773
2774
2775
2776
2777
2778
2779
2780
2781
2782
2783
2784
2785
2786
2787
2788
2789
2790
2791
2792
2793
2794
2795
2796
2797
2798
2799
2800
2801
2802
2803
2804
2805
2806
2807
2808
2809
2810
2811
2812
2813
2814
2815
2816
2817
2818
2819
2820
2821
2822
2823
2824
2825
2826
2827
2828
2829
2830
2831
2832
2833
2834
2835
2836
2837
2838
2839
2840
2841
2842
2843
2844
2845
2846
2847
2848
2849
2850
2851
2852
2853
2854
2855
2856
2857
2858
2859
2860
2861
2862
2863
2864
2865
2866
2867
2868
2869
2870
2871
2872
2873
2874
2875
2876
2877
2878
2879
2880
2881
2882
2883
2884
2885
2886
2887
2888
2889
2890
2891
2892
2893
2894
2895
2896
2897
2898
2899
2900
2901
2902
2903
2904
2905
2906
2907
2908
2909
2910
2911
2912
2913
2914
2915
2916
2917
2918
2919
2920
2921
2922
2923
2924
2925
2926
2927
2928
2929
2930
2931
2932
2933
2934
2935
2936
2937
2938
2939
2940
2941
2942
2943
2944
2945
2946
2947
2948
2949
2950
2951
2952
2953
2954
2955
2956
2957
2958
2959
2960
2961
2962
2963
2964
2965
2966
2967
2968
2969
2970
2971
2972
2973
2974
2975
2976
2977
2978
2979
2980
2981
2982
2983
2984
2985
2986
2987
2988
2989
2990
2991
2992
2993
2994
2995
2996
2997
2998
2999
3000
3001
3002
3003
3004
3005
3006
3007
3008
3009
3010
3011
3012
3013
3014
3015
3016
3017
3018
3019
3020
3021
3022
3023
3024
3025
3026
3027
3028
3029
3030
3031
3032
3033
3034
3035
3036
3037
3038
3039
3040
3041
3042
3043
3044
3045
3046
3047
3048
3049
3050
3051
3052
3053
3054
3055
3056
3057
3058
3059
3060
3061
3062
3063
3064
3065
3066
3067
3068
3069
3070
3071
3072
3073
3074
3075
3076
3077
3078
3079
3080
3081
3082
3083
3084
3085
3086
3087
3088
3089
3090
3091
3092
3093
3094
3095
3096
3097
3098
3099
3100
3101
3102
3103
3104
3105
3106
3107
3108
3109
3110
3111
3112
3113
3114
3115
3116
3117
3118
3119
3120
3121
3122
3123
3124
3125
3126
3127
3128
3129
3130
3131
3132
3133
3134
3135
3136
3137
3138
3139
3140
3141
3142
3143
3144
3145
3146
3147
3148
3149
3150
3151
3152
3153
3154
3155
3156
3157
3158
3159
3160
3161
3162
3163
3164
3165
3166
3167
3168
3169
3170
3171
3172
3173
3174
3175
3176
3177
3178
3179
3180
3181
3182
3183
3184
3185
3186
3187
3188
3189
3190
3191
3192
3193
3194
3195
3196
3197
3198
3199
3200
3201
3202
3203
3204
3205
3206
3207
3208
3209
3210
3211
3212
3213
3214
3215
3216
3217
3218
3219
3220
3221
3222
3223
3224
3225
3226
3227
3228
3229
3230
3231
3232
3233
3234
3235
3236
3237
3238
3239
3240
3241
3242
3243
3244
3245
3246
3247
3248
3249
3250
3251
3252
3253
3254
3255
3256
3257
3258
3259
3260
3261
3262
3263
3264
3265
3266
3267
3268
3269
3270
3271
3272
3273
3274
3275
3276
3277
3278
3279
3280
3281
3282
3283
3284
3285
3286
3287
3288
3289
3290
3291
3292
3293
3294
3295
3296
3297
3298
3299
3300
3301
3302
3303
3304
3305
3306
3307
3308
3309
3310
3311
3312
3313
3314
3315
3316
3317
3318
3319
3320
3321
3322
3323
3324
3325
3326
3327
3328
3329
3330
3331
3332
3333
3334
3335
3336
3337
3338
3339
3340
3341
3342
3343
3344
3345
3346
3347
3348
3349
3350
3351
3352
3353
3354
3355
3356
3357
3358
3359
3360
3361
3362
3363
3364
3365
3366
3367
3368
3369
3370
3371
3372
3373
3374
3375
3376
3377
3378
3379
3380
3381
3382
3383
3384
3385
3386
3387
3388
3389
3390
3391
3392
3393
3394
3395
3396
3397
3398
3399
3400
3401
3402
3403
3404
3405
3406
3407
3408
3409
3410
3411
3412
3413
3414
3415
3416
3417
3418
3419
3420
3421
3422
3423
3424
3425
3426
3427
3428
3429
3430
3431
3432
3433
3434
3435
3436
3437
3438
3439
3440
3441
3442
3443
3444
3445
3446
3447
3448
3449
3450
3451
3452
3453
3454
3455
3456
3457
3458
3459
3460
3461
3462
3463
3464
3465
3466
3467
3468
3469
3470
3471
3472
3473
3474
3475
3476
3477
3478
3479
3480
3481
3482
3483
3484
3485
3486
3487
3488
3489
3490
3491
3492
3493
3494
3495
3496
3497
3498
3499
3500
3501
3502
3503
3504
3505
3506
3507
3508
3509
3510
3511
3512
3513
3514
3515
3516
3517
3518
3519
3520
3521
3522
3523
3524
3525
3526
3527
3528
3529
3530
3531
3532
3533
3534
3535
3536
3537
3538
3539
3540
3541
3542
3543
3544
3545
3546
3547
3548
3549
3550
3551
3552
3553
3554
3555
3556
3557
3558
3559
3560
3561
3562
3563
3564
3565
3566
3567
3568
3569
3570
3571
3572
3573
3574
3575
3576
3577
3578
3579
3580
3581
3582
3583
3584
3585
3586
3587
3588
3589
3590
3591
3592
3593
3594
3595
3596
3597
3598
3599
3600
3601
3602
3603
3604
3605
3606
3607
3608
3609
3610
3611
3612
3613
3614
3615
3616
3617
3618
3619
3620
3621
3622
3623
3624
3625
3626
3627
3628
3629
3630
3631
3632
3633
3634
3635
3636
3637
3638
3639
3640
3641
3642
3643
3644
3645
3646
3647
3648
3649
3650
3651
3652
3653
3654
3655
3656
3657
3658
3659
3660
3661
3662
3663
3664
3665
3666
3667
3668
3669
3670
3671
3672
3673
3674
3675
3676
3677
3678
3679
3680
3681
3682
3683
3684
3685
3686
3687
3688
3689
3690
3691
3692
3693
3694
3695
3696
3697
3698
3699
3700
3701
3702
3703
3704
3705
3706
3707
3708
3709
3710
3711
3712
3713
3714
3715
3716
3717
3718
3719
3720
3721
3722
3723
3724
3725
3726
3727
3728
3729
3730
3731
3732
3733
3734
3735
3736
3737
3738
3739
3740
3741
3742
3743
3744
3745
3746
3747
3748
3749
3750
3751
3752
3753
3754
3755
3756
3757
3758
3759
3760
3761
3762
3763
3764
3765
3766
3767
3768
3769
3770
3771
3772
3773
3774
3775
3776
3777
3778
3779
3780
3781
3782
3783
3784
3785
3786
3787
3788
3789
3790
3791
3792
3793
3794
3795
3796
3797
3798
3799
3800
3801
3802
3803
3804
3805
3806
3807
3808
3809
3810
3811
3812
3813
3814
3815
3816
3817
3818
3819
3820
3821
3822
3823
3824
3825
3826
3827
3828
3829
3830
3831
3832
3833
3834
3835
3836
3837
3838
3839
3840
3841
3842
3843
3844
3845
3846
3847
3848
3849
3850
3851
3852
3853
3854
3855
3856
3857
3858
3859
3860
3861
3862
3863
3864
3865
3866
3867
3868
3869
3870
3871
3872
3873
3874
3875
3876
3877
3878
3879
3880
3881
3882
3883
3884
3885
3886
3887
3888
3889
3890
3891
3892
3893
3894
3895
3896
3897
3898
3899
3900
3901
3902
3903
3904
3905
3906
3907
3908
3909
3910
3911
3912
3913
3914
3915
3916
3917
3918
3919
3920
3921
3922
3923
3924
3925
3926
3927
3928
3929
3930
3931
3932
3933
3934
3935
3936
3937
3938
3939
3940
3941
3942
3943
3944
3945
3946
3947
3948
3949
3950
3951
3952
3953
3954
3955
3956
3957
3958
3959
3960
3961
3962
3963
3964
3965
3966
3967
3968
3969
3970
3971
3972
3973
3974
3975
3976
3977
3978
3979
3980
3981
3982
3983
3984
3985
3986
3987
3988
3989
3990
3991
3992
3993
3994
3995
3996
3997
3998
3999
4000
4001
4002
4003
4004
4005
4006
4007
4008
4009
4010
4011
4012
4013
4014
4015
4016
4017
4018
4019
4020
4021
4022
4023
4024
4025
4026
4027
4028
4029
4030
4031
4032
4033
4034
4035
4036
4037
4038
4039
4040
4041
4042
4043
4044
4045
4046
4047
4048
4049
4050
4051
4052
4053
4054
4055
4056
4057
4058
4059
4060
4061
4062
4063
4064
4065
4066
4067
4068
4069
4070
4071
4072
4073
4074
4075
4076
4077
4078
4079
4080
4081
4082
4083
4084
4085
4086
4087
4088
4089
4090
4091
4092
4093
4094
4095
4096
4097
4098
4099
4100
4101
4102
4103
4104
4105
4106
4107
4108
4109
4110
4111
4112
4113
4114
4115
4116
4117
4118
4119
4120
4121
4122
4123
4124
4125
4126
4127
4128
4129
4130
4131
4132
4133
4134
4135
4136
4137
4138
4139
4140
4141
4142
4143
4144
4145
4146
4147
4148
4149
4150
4151
4152
4153
4154
4155
4156
4157
4158
4159
4160
4161
4162
4163
4164
4165
4166
4167
4168
4169
4170
4171
4172
4173
4174
4175
4176
4177
4178
4179
4180
4181
4182
4183
4184
4185
4186
4187
4188
4189
4190
4191
4192
4193
4194
4195
4196
4197
4198
4199
4200
4201
4202
4203
4204
4205
4206
4207
4208
4209
4210
4211
4212
4213
4214
4215
4216
4217
4218
4219
4220
4221
4222
4223
4224
4225
4226
4227
4228
4229
4230
4231
4232
4233
4234
4235
4236
4237
4238
4239
4240
4241
4242
4243
4244
4245
4246
4247
4248
4249
4250
4251
4252
4253
4254
4255
4256
4257
4258
4259
4260
4261
4262
4263
4264
4265
4266
4267
4268
4269
4270
4271
4272
4273
4274
4275
4276
4277
4278
4279
4280
4281
4282
4283
4284
4285
4286
4287
4288
4289
4290
4291
4292
4293
4294
4295
4296
4297
4298
4299
4300
4301
4302
4303
4304
4305
4306
4307
4308
4309
4310
4311
4312
4313
4314
4315
4316
4317
4318
4319
4320
4321
4322
4323
4324
4325
4326
4327
4328
4329
4330
4331
4332
4333
4334
4335
4336
4337
4338
4339
4340
4341
4342
4343
4344
4345
4346
4347
4348
4349
4350
4351
4352
4353
4354
4355
4356
4357
4358
4359
4360
4361
4362
4363
4364
4365
4366
4367
4368
4369
4370
4371
4372
4373
4374
4375
4376
4377
4378
4379
4380
4381
4382
4383
4384
4385
4386
4387
4388
4389
4390
4391
4392
4393
4394
4395
4396
4397
4398
4399
4400
4401
4402
4403
4404
4405
4406
4407
4408
4409
4410
4411
4412
4413
4414
4415
4416
4417
4418
4419
4420
4421
4422
4423
4424
4425
4426
4427
4428
4429
4430
4431
4432
4433
4434
4435
4436
4437
4438
4439
4440
4441
4442
4443
4444
4445
4446
4447
4448
4449
4450
4451
4452
4453
4454
4455
4456
4457
4458
4459
4460
4461
4462
4463
4464
4465
4466
4467
4468
4469
4470
4471
4472
4473
4474
4475
4476
4477
4478
4479
4480
4481
4482
4483
4484
4485
4486
4487
4488
4489
4490
4491
4492
4493
4494
4495
4496
4497
4498
4499
4500
4501
4502
4503
4504
4505
4506
4507
4508
4509
4510
4511
4512
4513
4514
4515
4516
4517
4518
4519
4520
4521
4522
4523
4524
4525
4526
4527
4528
4529
4530
4531
4532
4533
4534
4535
4536
4537
4538
4539
4540
4541
4542
4543
4544
4545
4546
4547
4548
4549
4550
4551
4552
4553
4554
4555
4556
4557
4558
4559
4560
4561
4562
4563
4564
4565
4566
4567
4568
4569
4570
4571
4572
4573
4574
4575
4576
4577
4578
4579
4580
4581
4582
4583
4584
4585
4586
4587
4588
4589
4590
4591
4592
4593
4594
4595
4596
4597
4598
4599
4600
4601
4602
4603
4604
4605
4606
4607
4608
4609
4610
4611
4612
4613
4614
4615
4616
4617
4618
4619
4620
4621
4622
4623
4624
4625
4626
4627
4628
4629
4630
4631
4632
4633
4634
4635
4636
4637
4638
4639
4640
4641
4642
4643
4644
4645
4646
4647
4648
4649
4650
4651
4652
4653
4654
4655
4656
4657
4658
4659
4660
4661
4662
4663
4664
4665
4666
4667
4668
4669
4670
4671
4672
4673
4674
4675
4676
4677
4678
4679
4680
4681
4682
4683
4684
4685
4686
4687
4688
4689
4690
4691
4692
4693
4694
4695
4696
4697
4698
4699
4700
4701
4702
4703
4704
4705
4706
4707
4708
4709
4710
4711
4712
4713
4714
4715
4716
4717
4718
4719
4720
4721
4722
4723
4724
4725
4726
4727
4728
4729
4730
4731
4732
4733
4734
4735
4736
4737
4738
4739
4740
4741
4742
4743
4744
4745
4746
4747
4748
4749
4750
4751
4752
4753
4754
4755
4756
4757
4758
4759
4760
4761
4762
4763
4764
4765
4766
4767
4768
4769
4770
4771
4772
4773
4774
4775
4776
4777
4778
4779
4780
4781
4782
4783
4784
4785
4786
4787
4788
4789
4790
4791
4792
4793
4794
4795
4796
4797
4798
4799
4800
4801
4802
4803
4804
4805
4806
4807
4808
4809
4810
4811
4812
4813
4814
4815
4816
4817
4818
4819
4820
4821
4822
4823
4824
4825
4826
4827
4828
4829
4830
4831
4832
4833
4834
4835
4836
4837
4838
4839
4840
4841
4842
4843
4844
4845
4846
4847
4848
4849
4850
4851
4852
4853
4854
4855
4856
4857
4858
4859
4860
4861
4862
4863
4864
4865
4866
4867
4868
4869
4870
4871
4872
4873
4874
4875
4876
4877
4878
4879
4880
4881
4882
4883
4884
4885
4886
4887
4888
4889
4890
4891
4892
4893
4894
4895
4896
4897
4898
4899
4900
4901
4902
4903
4904
4905
4906
4907
4908
4909
4910
4911
4912
4913
4914
4915
4916
4917
4918
4919
4920
4921
4922
4923
4924
4925
4926
4927
4928
4929
4930
4931
4932
4933
4934
4935
4936
4937
4938
4939
4940
4941
4942
4943
4944
4945
4946
4947
4948
4949
4950
4951
4952
4953
4954
4955
4956
4957
4958
4959
4960
4961
4962
4963
4964
4965
4966
4967
4968
4969
4970
4971
4972
4973
4974
4975
4976
4977
4978
4979
4980
4981
4982
4983
4984
4985
4986
4987
4988
4989
4990
4991
4992
4993
4994
4995
4996
4997
4998
4999
5000
--- RAW OUTPUT END ---

## Bash — Output over size threshold (seq 1 12000, 59.5KB)

**Call:** `Bash({command: "seq 1 12000", description: "Print numbers 1 to 12000 to probe output truncation"})`

--- RAW OUTPUT START ---
[persisted-output]
Output too large (59.5KB). Full output saved to: C:\Users\tgurt\.claude\projects\C--AI-MOSAIC-MOSAIC\2149b7a2-fa05-489e-b5b7-cdc04b7e7d32\tool-results\b0vly603g.txt

Preview (first 2KB):
1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21
22
23
24
25
26
27
28
29
30
31
32
33
34
35
36
37
38
39
40
41
42
43
44
45
46
47
48
49
50
51
52
53
54
55
56
57
58
59
60
61
62
63
64
65
66
67
68
69
70
71
72
73
74
75
76
77
78
79
80
81
82
83
84
85
86
87
88
89
90
91
92
93
94
95
96
97
98
99
100
101
102
103
104
105
106
107
108
109
110
111
112
113
114
115
116
117
118
119
120
121
122
123
124
125
126
127
128
129
130
131
132
133
134
135
136
137
138
139
140
141
142
143
144
145
146
147
148
149
150
151
152
153
154
155
156
157
158
159
160
161
162
163
164
165
166
167
168
169
170
171
172
173
174
175
176
177
178
179
180
181
182
183
184
185
186
187
188
189
190
191
192
193
194
195
196
197
198
199
200
201
202
203
204
205
206
207
208
209
210
211
212
213
214
215
216
217
218
219
220
221
222
223
224
225
226
227
228
229
230
231
232
233
234
235
236
237
238
239
240
241
242
243
244
245
246
247
248
249
250
251
252
253
254
255
256
257
258
259
260
261
262
263
264
265
266
267
268
269
270
271
272
273
274
275
276
277
278
279
280
281
282
283
284
285
286
287
288
289
290
291
292
293
294
295
296
297
298
299
300
301
302
303
304
305
306
307
308
309
310
311
312
313
314
315
316
317
318
319
320
321
322
323
324
325
326
327
328
329
330
331
332
333
334
335
336
337
338
339
340
341
342
343
344
345
346
347
348
349
350
351
352
353
354
355
356
357
358
359
360
361
362
363
364
365
366
367
368
369
370
371
372
373
374
375
376
377
378
379
380
381
382
383
384
385
386
387
388
389
390
391
392
393
394
395
396
397
398
399
400
401
402
403
404
405
406
407
408
409
410
411
412
413
414
415
416
417
418
419
420
421
422
423
424
425
426
427
428
429
430
431
432
433
434
435
436
437
438
439
440
441
442
443
444
445
446
447
448
449
450
451
452
453
454
455
456
457
458
459
460
461
462
463
464
465
466
467
468
469
470
471
472
473
474
475
476
477
478
479
480
481
482
483
484
485
486
487
488
489
490
491
492
493
494
495
496
497
498
499
500
501
502
503
504
505
506
507
508
509
510
511
512
513
514
515
516
517
518
519
520
521
522
523
524
525
526
527
...
[/persisted-output]
--- RAW OUTPUT END ---

## TaskStop — Error: unknown task ID

**Call:** `TaskStop({task_id: "nonexistent-task-id-123"})`

--- RAW OUTPUT START ---
[error][tool_use_error]No task found with ID: nonexistent-task-id-123[/tool_use_error][/error]
--- RAW OUTPUT END ---

## AskUserQuestion — Not captured: tool is not present in the subagent tool list (no ask-user tool is exposed to subagents in this harness version)

## Agent (task/spawn) — Not captured: guide restricts the spawn-agent capture to the PrimaryAgent tier

## SubagentHandback — Not captured: calling it ends the subagent run; its result is delivered to the caller, not returned to the calling agent
