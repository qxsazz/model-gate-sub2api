import {describe,it,expect} from 'vitest'
import {parse,compileStyle} from 'vue/compiler-sfc'
import source from '../user/AchievementsView.vue?raw'

function palette(dark:boolean){
 const wasDark=document.documentElement.classList.contains('dark')
 const root=document.createElement('div');root.className='achievement-page';root.setAttribute('data-v-theme-test','')
 const style=document.createElement('style')
 const sfc=parse(source).descriptor
 const css=compileStyle({source:sfc.styles[0]!.content,filename:'AchievementsView.vue',id:'data-v-theme-test',scoped:true})
 expect(css.errors).toEqual([])
 document.documentElement.classList.toggle('dark',dark)
 document.body.append(root);style.textContent=css.code;document.head.append(style)
 const values:Record<string,string>={}
 try{
  for(const rule of Array.from(style.sheet!.cssRules)){
   if(rule.type!==1)continue
   const cssRule=rule as CSSStyleRule
   for(const key of ['--paper','--ink']){
    const value=cssRule.style.getPropertyValue(key)
    if(value&&root.matches(cssRule.selectorText))values[key]=value.trim()
   }
  }
  return values
 }finally{root.remove();style.remove();document.documentElement.classList.toggle('dark',wasDark)}
}
describe('achievement palette in the compiled application',()=>{
 it('keeps bright paper and dark titles in light mode',()=>{
  const p=palette(false);expect(parseInt(p['--paper']!.slice(1,3),16)).toBeGreaterThan(180)
  expect(parseInt(p['--ink']!.slice(1,3),16)).toBeLessThan(100)
 })
 it('switches the actual component paper and title tokens in dark mode',()=>{
  const p=palette(true);expect(parseInt(p['--paper']!.slice(1,3),16)).toBeLessThan(100)
  expect(parseInt(p['--ink']!.slice(1,3),16)).toBeGreaterThan(180)
 })
})
