import test from 'node:test';
import assert from 'node:assert/strict';
import { sessionNotice, expiryNotice, installActivity } from './viewer-session.mjs';

test('only deliberate viewing completion covers the document', () => {
  for (const reason of ['timeout', 'disconnected']) assert.equal(sessionNotice(false, reason).cover, false);
  assert.equal(sessionNotice(false, 'closed').cover, true);
  for (const reason of ['timeout', 'disconnected', 'closed']) assert.equal(sessionNotice(true, reason).cover, true);
});
test('idle warning can extend; explicit deadlines cannot', () => {
  assert.deepEqual(expiryNotice({seconds_left:61,idle:true}), {text:'',extend:false});
  assert.equal(expiryNotice({seconds_left:60,idle:true}).extend,true);
  assert.equal(expiryNotice({seconds_left:20,idle:false}).extend,false);
  assert.equal(expiryNotice({reason:'closed',seconds_left:0,idle:true}).text,'');
});
test('activity only comes from throttled trusted input and is removable', () => {
  const listeners = new Map();
  const target = {addEventListener:(k,v)=>listeners.set(k,v), removeEventListener:k=>listeners.delete(k)};
  let clock=0, calls=0;
  const stop=installActivity(target,()=>calls++,()=>clock);
  listeners.get('keydown')({isTrusted:false});
  assert.equal(calls,0);
  listeners.get('keydown')({isTrusted:true});
  listeners.get('wheel')({isTrusted:true});
  assert.equal(calls,1);
  clock=3000;
  listeners.get('pointerdown')({isTrusted:true});
  assert.equal(calls,2);
  assert.equal(listeners.has('message'),false);
  stop();assert.equal(listeners.size,0);
});
