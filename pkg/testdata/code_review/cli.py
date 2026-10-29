import json, os, pathlib, subprocess
root=pathlib.Path(__file__).resolve().parents[3]
folder=root/'pkg/build/code_review_cli'
folder.mkdir(parents=True,exist_ok=True)
executables={'scala':pathlib.Path(os.environ.get('APALACHE_SCALA', root/'target/universal/stage/bin/apalache-mc')),'go':pathlib.Path(os.environ.get('APALACHE_GO', root/'pkg/build/apalache'))}
checks={
 'legacy_gen_equality': 'LET (* @type: [a: Int]; *) R == Gen(1) IN [R EXCEPT !.a = 0] = [a |-> 0]',

 'legacy_gen_domain': 'LET (* @type: [a: Int]; *) R == Gen(1) IN DOMAIN R = {"a"}',

 'nested_default_subseq': 'SubSeq(<< <<[i \\in {1} |-> i], 2>> >>, 1, x)[1][2] = 2',
 'nested_default_concat': '(SubSeq(<< <<[i \\in {1} |-> i], 2>> >>, 1, x) \\o << <<[i \\in {1} |-> i], 2>> >>)[2][2] = 2',

 'import_local_annotations': 'Foo = 1 /\\ Bar = \"s\"',
 'nested_default_function': 'Head(<< <<[i \\in {1} |-> i], 2>> >>)[2] = 2',
 'parser_user_print': 'Print(FALSE, TRUE)',
 'parser_user_variant': 'Variant(1,2) = 3',
 'parser_user_override': '(3 @@ 2) = 1',
 'legacy_record_except': 'LET r == IF x = 2 THEN [a |-> 1] ELSE [a |-> 1, b |-> 2] IN [r EXCEPT !.b = 3] = r',
 'range_function_false_proof': '[i \\in 1..x |-> i][1] = 0',
 'range_cup': '((1..x) \\cup {3}) = {1,2,3}',
 'range_cup_false_proof': '((1..x) \\cup {3}) = {3}',
 'range_function': '[i \\in 1..x |-> i][1] = 1',
 'range_function_domain': 'DOMAIN [i \\in 1..x |-> i] = {1,2}',
 'range_map': '{i : i \\in 1..x} = {1,2}',
 'range_powerset': '{} \\in SUBSET (1..x)',
 'range_cardinality': 'Cardinality(1..x) = 2',
 'range_union': 'UNION {1..x} = {1,2}',
 'range_eq': '(1..x) = {1,2}',
 'function_duplicate_keys': '[i \\in {1,1} |-> i+1][1] = 2',
 'powerset_duplicate': 'Cardinality(Expand(SUBSET {1,1})) = 2',
 'function_empty_codomain': '[{} -> {}] = {[i \\in {} |-> 1]}',
 'seq_tail_empty': 'Len(Tail(SubSeq(<<1>>,2,1))) = 0',
 'seq_subseq_clip': 'SubSeq(<<1,2>>,0,9) = <<1,2>>',
 'negative_mod': '(-5) % 3 = 1',
 'negative_div': '(-5) \\div 3 = -2',
 'negative_power': '2^(-1) = 0',
 'zero_power': '0^0 = 1',
}
records={}
for name, inv in checks.items():
 if name in records: continue
 source='---- MODULE Audit ----\nEXTENDS Integers, Sequences, FiniteSets, Apalache\nVARIABLE\n\\* @type: Int;\nx\nInit == x = 2\nNext == x\' = x\nInv == '+inv+'\n====\n'
 if name=='import_local_annotations':
  source=source.replace('EXTENDS Integers,', 'EXTENDS Helper, Integers,')
  (folder/'Helper.tla').write_text('---- MODULE Helper ----\nFoo == LET (* @type: Int; *) Local == 1 IN Local\nBar == LET (* @type: Str; *) Local == \"s\" IN Local\n====\n')
 if name=='parser_user_print': source=source.replace('VARIABLE\n','T == INSTANCE TLC\nPrint(a,b) == a\nVARIABLE\n')
 if name=='parser_user_variant': source=source.replace('VARIABLE\n','V == INSTANCE Variants\nVariant(a,b) == a+b\nVARIABLE\n')
 if name=='parser_user_override': source=source.replace('VARIABLE\n','a @@ b == a-b\nVARIABLE\n')
 path=folder/'Audit.tla';path.write_text(source)
 results={}
 for language,exe in executables.items():
  args=[str(exe),'check','--length=0','--inv=Inv',str(path)]
  if name.startswith('legacy_'): args.insert(2,'--features=no-rows')
  try:
   p=subprocess.run(['taskset','-c','1-8']+args,cwd=folder,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=40)
   results[language]={'exit':p.returncode,'output':p.stdout}
  except subprocess.TimeoutExpired:results[language]={'exit':'timeout'}
 records[name]={'source':source,**results}
 print(name,{k:v['exit'] for k,v in results.items()},flush=True)
 (folder/'results.json').write_text(json.dumps(records,indent=2))
