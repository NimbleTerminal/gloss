#!/usr/bin/env python3
"""Build pinned font subsets. Requires fonttools==4.60.2; never used at runtime.
Usage: python scripts/subset-fonts.py CACHE [--download]
The download step verifies sources.json hashes before instancing/subsetting.
"""
import argparse, hashlib, json, urllib.request, zipfile
from pathlib import Path
from fontTools import subset
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
import fontTools

assert fontTools.__version__ == '4.60.2', 'install fonttools==4.60.2'
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('cache',type=Path)
parser.add_argument('--download',action='store_true')
args=parser.parse_args()
root=Path(__file__).resolve().parents[1]/'internal/document/fonts'
args.cache.mkdir(parents=True,exist_ok=True)
for source in json.loads((root/'sources.json').read_text()):
 path=args.cache/source['file']
 if not path.exists() and args.download:
  with urllib.request.urlopen(source['url'],timeout=120) as response:path.write_bytes(response.read())
 data=path.read_bytes()
 assert hashlib.sha256(data).hexdigest()==source['sha256'], f'checksum mismatch: {path}'
 if source['file']=='iosevka.zip':
  with zipfile.ZipFile(path) as archive:
   for variant in ['Regular','Bold','Italic','BoldItalic']:
    name='Iosevka-'+variant+'.ttf';(args.cache/name).write_bytes(archive.read(name))
# Latin/Latin-1, combining accents, general punctuation, and the replacement glyph.
# No CJK, emoji, Greek/Cyrillic, or discretionary ligature tables are shipped.
unicodes=set(range(0x20,0x7f))|set(range(0xa0,0x100))|set(range(0x300,0x370))|set(range(0x2000,0x2070))|{0xfffd}
families=[('A','literata','Literata','opsz,wght',12),('B','newsreader','Newsreader','opsz,wght',16),('C','ibmplexsans','IBMPlexSans','wdth,wght',None),('D','sourceserif4','SourceSerif4','opsz,wght',16),('E','iosevka','Iosevka',None,None),('F','ibmplexmono','IBMPlexMono',None,None),('G','jetbrainsmono','JetBrainsMono','wght',None)]
hashes={}
for ident,folder,family,axes,optical in families:
 for variant,weight,italic in [('Regular',400,False),('Bold',700,False),('Italic',400,True),('BoldItalic',700,True)]:
  if axes:
   name=folder+'-'+family+('-Italic' if italic else '')+'['+axes+'].ttf'
  elif folder=='iosevka':name=family+'-'+variant+'.ttf'
  else:name=folder+'-'+family+'-'+variant+'.ttf'
  font=TTFont(args.cache/name,recalcTimestamp=False)
  if 'fvar' in font:
   settings={a.axisTag:a.defaultValue for a in font['fvar'].axes};settings['wght']=weight
   if optical is not None:settings['opsz']=optical
   font=instantiateVariableFont(font,settings,inplace=True)
  options=subset.Options();options.hinting=False
  options.layout_features=['ccmp','locl','mark','mkmk','kern']
  options.name_IDs=['*'];options.name_legacy=True;options.name_languages=['*']
  sub=subset.Subsetter(options=options);sub.populate(unicodes=unicodes);sub.subset(font)
  familyname='Gloss Text '+ident
  names={1:familyname,2:variant,3:'GlossSubset-'+ident+'-'+variant,4:familyname+' '+variant,6:'GlossText'+ident+'-'+variant,16:familyname,17:variant,21:familyname,22:variant}
  for record in font['name'].names:
   if record.nameID in names:record.string=names[record.nameID].encode(record.getEncoding(),errors='replace')
  font['head'].created=font['head'].modified=2082844800
  dest=root/('Gloss'+ident+'-'+variant+'.ttf');font.save(dest,reorderTables=True)
  hashes[dest.name]=hashlib.sha256(dest.read_bytes()).hexdigest()
(root/'subsets.sha256').write_text(''.join(f'{digest}  {name}\n' for name,digest in sorted(hashes.items())))
print('Subset bytes:',sum((root/n).stat().st_size for n in hashes))
